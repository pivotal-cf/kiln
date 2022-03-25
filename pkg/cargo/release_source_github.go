package cargo

type GitHubReleaseSource struct {
	_ struct{} `yaml:"-"`

	Publishable bool   `yaml:"publishable"`
	Identifier  string `yaml:"id"`

	Org string `yaml:"org"`

	// secrets

	GithubToken string `yaml:"github_token"`
}

func (rs GitHubReleaseSource) Type() string { return ReleaseSourceTypeGithub }

func (rs GitHubReleaseSource) ID() string {
	if rs.Identifier != "" {
		return rs.Identifier
	}
	return rs.Org
}

func (rs GitHubReleaseSource) IsPublishable() bool {
	return rs.Publishable
}

func (rs GitHubReleaseSource) ConfigureSecrets(tv TemplateVariables) (ReleaseSource, error) {
	var err error
	rs.GithubToken, err = configureSecret(rs.GithubToken, "github_token", "GITHUB_TOKEN", tv)
	return rs, err
}
