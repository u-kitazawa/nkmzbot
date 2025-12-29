package geourl

import (
	"testing"
)

func TestExtractFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantLat float64
		wantLng float64
		wantOk  bool
	}{
		{
			name:    "Pattern A: @lat,lng format",
			url:     "https://www.google.com/maps/@35.696677,138.430228,15z",
			wantLat: 35.696677,
			wantLng: 138.430228,
			wantOk:  true,
		},
		{
			name:    "Pattern B: !3dlat!4dlng format",
			url:     "https://www.google.com/maps/place/Tokyo!3d35.6762!4d139.6503",
			wantLat: 35.6762,
			wantLng: 139.6503,
			wantOk:  true,
		},
		{
			name:    "Pattern C: /maps/search/lat,lng format with plus",
			url:     "https://www.google.com/maps/search/35.696677,+138.430228?coh=277533&entry=tts",
			wantLat: 35.696677,
			wantLng: 138.430228,
			wantOk:  true,
		},
		{
			name:    "Pattern C: /maps/search/lat,lng format without plus",
			url:     "https://www.google.com/maps/search/35.696677,138.430228",
			wantLat: 35.696677,
			wantLng: 138.430228,
			wantOk:  true,
		},
		{
			name:    "Pattern C: /maps/search/lat,lng with space separator",
			url:     "https://www.google.com/maps/search/35.696677, 138.430228",
			wantLat: 35.696677,
			wantLng: 138.430228,
			wantOk:  true,
		},
		{
			name:    "Pattern C: /maps/search/lat,lng with negative longitude",
			url:     "https://www.google.com/maps/search/35.696677,-138.430228",
			wantLat: 35.696677,
			wantLng: -138.430228,
			wantOk:  true,
		},
		{
			name:    "Pattern D: query param ?q=lat,lng",
			url:     "https://www.google.com/maps?q=35.696677,138.430228",
			wantLat: 35.696677,
			wantLng: 138.430228,
			wantOk:  true,
		},
		{
			name:    "Pattern D: query param ?query=lat,lng",
			url:     "https://www.google.com/maps?query=35.696677, 138.430228",
			wantLat: 35.696677,
			wantLng: 138.430228,
			wantOk:  true,
		},
		{
			name:    "Negative coordinates",
			url:     "https://www.google.com/maps/search/-33.8688,+151.2093",
			wantLat: -33.8688,
			wantLng: 151.2093,
			wantOk:  true,
		},
		{
			name:    "No coordinates",
			url:     "https://www.google.com/maps/place/Tokyo",
			wantLat: 0,
			wantLng: 0,
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLng, gotOk := extractFromURL(tt.url)
			if gotOk != tt.wantOk {
				t.Errorf("extractFromURL() gotOk = %v, want %v", gotOk, tt.wantOk)
				return
			}
			if !tt.wantOk {
				return
			}
			if gotLat != tt.wantLat {
				t.Errorf("extractFromURL() gotLat = %v, want %v", gotLat, tt.wantLat)
			}
			if gotLng != tt.wantLng {
				t.Errorf("extractFromURL() gotLng = %v, want %v", gotLng, tt.wantLng)
			}
		})
	}
}
