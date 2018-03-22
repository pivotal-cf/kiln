package cargo

type Manifest struct {
	Name      string
	Releases  []Release
	Stemcells []Stemcell
	Update    Update
}

type Release struct {
	Name    string
	Version string
}

type Stemcell struct {
	Alias   string
	OS      string
	Version string
}

type Update struct {
	Canaries        int
	CanaryWatchTime string
	UpdateWatchTime string
	MaxInFlight     int
	MaxErrors       int
	Serial          bool
}
