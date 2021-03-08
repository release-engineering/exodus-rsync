package conf

type sharedConfig struct {
	GwEnvRaw          string `yaml:"gwenv"`
	GwCertRaw         string `yaml:"gwcert"`
	GwKeyRaw          string `yaml:"gwkey"`
	GwURLRaw          string `yaml:"gwurl"`
	GwPollIntervalRaw int    `yaml:"gwpollinterval"`
	GwBatchSizeRaw    int    `yaml:"gwbatchsize"`
}

type environment struct {
	sharedConfig `yaml:",inline"`

	PrefixRaw string `yaml:"prefix"`

	parent *globalConfig
}

type globalConfig struct {
	sharedConfig `yaml:",inline"`

	// Configuration for each environment.
	EnvironmentsRaw []environment `yaml:"environments"`
}

func (g *globalConfig) GwCert() string {
	return g.GwCertRaw
}

func (g *globalConfig) GwKey() string {
	return g.GwKeyRaw
}

func (g *globalConfig) GwURL() string {
	return g.GwURLRaw
}

func (g *globalConfig) GwEnv() string {
	return g.GwEnvRaw
}

func (g *globalConfig) GwPollInterval() int {
	return g.GwPollIntervalRaw
}

func (g *globalConfig) GwBatchSize() int {
	return g.GwBatchSizeRaw
}

func nonEmptyString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func nonEmptyInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func (e *environment) GwCert() string {
	return nonEmptyString(e.GwCertRaw, e.parent.GwCertRaw)
}

func (e *environment) GwKey() string {
	return nonEmptyString(e.GwKeyRaw, e.parent.GwKeyRaw)
}

func (e *environment) GwURL() string {
	return nonEmptyString(e.GwURLRaw, e.parent.GwURLRaw)
}

func (e *environment) GwEnv() string {
	return nonEmptyString(e.GwEnvRaw, e.parent.GwEnvRaw)
}

func (e *environment) GwPollInterval() int {
	return nonEmptyInt(e.GwPollIntervalRaw, e.parent.GwPollIntervalRaw)
}

func (e *environment) GwBatchSize() int {
	return nonEmptyInt(e.GwBatchSizeRaw, e.parent.GwBatchSizeRaw)
}

func (e *environment) Prefix() string {
	return e.PrefixRaw
}
