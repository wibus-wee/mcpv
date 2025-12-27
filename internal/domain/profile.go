package domain

const DefaultProfileName = "default"

type Profile struct {
	Name    string
	Catalog Catalog
}

type ProfileStore struct {
	Profiles map[string]Profile
	Callers  map[string]string
}
