package user

import "time"

type Profile struct {
	ID                  uint64
	Timezone            string
	LargeTextEnabled    bool
	HighContrastEnabled bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PreferencesPatch struct {
	Timezone            *string
	LargeTextEnabled    *bool
	HighContrastEnabled *bool
}
