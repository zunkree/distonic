package distonic

import (
	"log"
	"os"
	"path"
	"strings"
	"text/template"

	git "github.com/libgit2/git2go"
	"github.com/spf13/viper"
)

type Worker struct {
}

func NewWorker() (*Worker, error) {
	return &Worker{}, nil
}

func (w *Worker) Run(orders <-chan *Order) {
	for order := range orders {
		log.Printf("Received order: %s", order)
		err := w.processOrder(order)
		if err != nil {
			log.Printf("Error processing order `%s`: %s", order, err)
		}
	}
}

func (w *Worker) processOrder(order *Order) error {
	workdir, err := w.prepareWorkdir(order)
	if err != nil {
		log.Printf("Error preparing workdir for order `%s`: %s", order, err)
		return err
	}

	context := &Context{
		Workdir:      workdir,
		Branch:       order.branchName,
		BranchDashed: strings.Replace(order.branchName, "/", "-", -1),
		Commit:       order.commit.Object.Id().String()}

	pipeline, err := w.readPipeline(context)
	if err != nil {
		log.Printf("Could not read pipeline for order `%s`: %s", order, err)
		return err
	}

	log.Fatal(pipeline)

	return nil
}

func (w *Worker) prepareWorkdir(order *Order) (string, error) {
	var err error
	var repo *git.Repository

	dataDir := viper.GetString("data_dir")
	workDir := path.Join(
		dataDir,
		"worker",
		order.repoName,
		order.branchName,
		order.commit.Object.Id().String())

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		repo, err = git.Clone(
			order.repo.Path(),
			workDir,
			&git.CloneOptions{
				Bare:           false,
				CheckoutBranch: order.branchName,
				CheckoutOpts:   &git.CheckoutOpts{Strategy: git.CheckoutForce}})
		if err != nil {
			log.Printf(
				"Cannot make working clone for repo `%s`: %s",
				order.repoName, err)
			return "", err
		}
	} else {
		repo, err = git.OpenRepository(workDir)
		if err != nil {
			log.Printf(
				"Cannot open working clone for repo `%s`: %s",
				order.repoName, err)
			return "", err
		}
	}

	err = repo.SetHeadDetached(order.commit.Object.Id())
	if err != nil {
		log.Printf("Cannot set head on repo `%s`: %s", order.repoName, err)
		return "", err
	}

	err = repo.CheckoutHead(&git.CheckoutOpts{Strategy: git.CheckoutForce})
	if err != nil {
		log.Printf(
			"Cannot checkout workdir for repo `%s`: %s",
			order.repoName, err)
		return "", err
	}

	log.Printf("Working dir `%s` is ready", workDir)
	return workDir, nil
}

func (w *Worker) readPipeline(context *Context) (*Pipeline, error) {
	configName := "distonic"
	configFilename := path.Join(context.Workdir, "distonic.yml")

	t, err := template.ParseFiles(configFilename)
	if err != nil {
		log.Printf("Could not load distonic pipeline template: %s", err)
		return nil, err
	}

	config, err := os.Create(configFilename)
	if err != nil {
		log.Printf("Could not open distonic pipeline for writing: %s", err)
		return nil, err
	}

	if err := t.Execute(config, context); err != nil {
		log.Printf("Could not execute distonic pipeline template: %s", err)
		return nil, err
	}

	p := viper.New()
	p.SetConfigName(configName)
	p.AddConfigPath(context.Workdir)
	if err := p.ReadInConfig(); err != nil {
		log.Printf("Could not read distonic pipeline config: %s", err)
		return nil, err
	}

	pipeline, err := NewPipeline(p)
	if err != nil {
		log.Printf("Could not initialize pipeline: %s", err)
		return nil, err
	}
	return pipeline, nil
}
