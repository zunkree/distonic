package distonic

import (
	"log"

	"github.com/spf13/viper"
)

type BindableModule interface {
	Bind(params map[string]interface{}) (CallableModule, error)
}

type CallableModule interface {
	Call(context *Context) error
}

type Job struct {
	name   string
	module CallableModule
}

type Stage struct {
	name string
	jobs []*Job
}

type Pipeline struct {
	stages []*Stage
}

func NewPipeline(p *viper.Viper) (*Pipeline, error) {
	stages := []*Stage{}
	for _, stageName := range p.AllKeys() {
		jobs := []*Job{}
	jobs:
		for _, jobSpec := range p.Get(stageName).([]interface{}) {
			job := Job{}
			for k, v := range jobSpec.(map[interface{}]interface{}) {
				switch k {
				case "name":
					job.name = v.(string)
				default:
					module, err := getModule(k.(string))
					if err != nil {
						log.Print(err)
						continue jobs
					}
					job.module, err = module.Bind(v.(map[string]interface{}))
					if err != nil {
						log.Printf(
							"Could not initialize `%s` module: %s", k, err)
						return nil, err
					}
				}
			}
			jobs = append(jobs, &job)
		}
		stages = append(stages, &Stage{name: stageName, jobs: jobs})
	}
	return &Pipeline{stages: stages}, nil
}
