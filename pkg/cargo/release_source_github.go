package cargo

type GitHubReleaseSource struct {
	_ struct{} `yaml:"-"`

	Publishable bool   `yaml:"publishable"`
	Identifier  string `yaml:"id"`

	Org         string `yaml:"org"`
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
