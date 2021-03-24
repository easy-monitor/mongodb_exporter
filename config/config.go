package config

type Config struct {
	MongoModules []*MongoModule `yaml:"module",json:"module"`
}

type MongoModule struct {
	Name     string `yaml:"name",json:"name"`
	User     string `yaml:"user",json:"user"`
	Password string `yaml:"password",json:"password"`
}
