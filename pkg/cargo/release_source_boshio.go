package cargo

type BOSHIOReleaseSource struct {
	_ struct{} `yaml:"-"`

	Publishable bool   `yaml:"publishable"`
	Identifier  string `yaml:"id"`
}

func (rs BOSHIOReleaseSource) Type() string { return ReleaseSourceTypeBOSHIO }

func (rs BOSHIOReleaseSource) ID() string {
	if rs.Identifier != "" {
		return rs.Identifier
	}
	return rs.Type()
}

func (rs BOSHIOReleaseSource) IsPublishable() bool {
	return rs.Publishable
}
