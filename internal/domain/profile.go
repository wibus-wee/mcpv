package domain

// DefaultProfileName is the default profile name.
const DefaultProfileName = "default"

// Profile bundles a catalog under a named profile.
type Profile struct {
	Name    string
	Catalog Catalog
}

// ProfileStore contains profiles and caller mappings.
type ProfileStore struct {
	profiles map[string]Profile
	callers  map[string]string
}

// NewProfileStore constructs an immutable profile store snapshot.
func NewProfileStore(profiles map[string]Profile, callers map[string]string) ProfileStore {
	return ProfileStore{
		profiles: cloneProfiles(profiles),
		callers:  cloneCallers(callers),
	}
}

// Profiles returns a copy of the stored profiles.
func (p ProfileStore) Profiles() map[string]Profile {
	return cloneProfiles(p.profiles)
}

// Callers returns a copy of the stored caller mappings.
func (p ProfileStore) Callers() map[string]string {
	return cloneCallers(p.callers)
}

func cloneProfiles(src map[string]Profile) map[string]Profile {
	if len(src) == 0 {
		return map[string]Profile{}
	}
	out := make(map[string]Profile, len(src))
	for name, profile := range src {
		out[name] = cloneProfile(profile)
	}
	return out
}

func cloneProfile(profile Profile) Profile {
	out := profile
	out.Catalog = cloneCatalog(profile.Catalog)
	return out
}

func cloneCatalog(catalog Catalog) Catalog {
	out := catalog
	if len(catalog.Specs) == 0 {
		out.Specs = map[string]ServerSpec{}
		return out
	}
	out.Specs = make(map[string]ServerSpec, len(catalog.Specs))
	for key, spec := range catalog.Specs {
		out.Specs[key] = spec
	}
	return out
}

func cloneCallers(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for caller, profile := range src {
		out[caller] = profile
	}
	return out
}
