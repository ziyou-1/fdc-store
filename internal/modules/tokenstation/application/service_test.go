package application

import (
	"testing"

	tokenstationdomain "github.com/dujiao-next/internal/modules/tokenstation/domain"
)

type stubRepo struct {
	stations []tokenstationdomain.Station
}

func (r *stubRepo) ListStations() ([]tokenstationdomain.Station, error) { return r.stations, nil }
func (r *stubRepo) GetStation(id uint) (*tokenstationdomain.Station, error) {
	for i := range r.stations {
		if r.stations[i].ID == id {
			return &r.stations[i], nil
		}
	}
	return nil, nil
}
func (r *stubRepo) CreateStation(*tokenstationdomain.Station) error { return nil }
func (r *stubRepo) UpdateStation(*tokenstationdomain.Station) error { return nil }
func (r *stubRepo) DeleteStation(uint) error                        { return nil }

func TestPublicCatalogListsRemoteLinks(t *testing.T) {
	svc := NewService(&stubRepo{stations: []tokenstationdomain.Station{
		{ID: 1, Name: "对方站", BaseURL: "https://relay.example.com"},
	}})
	catalog, err := svc.PublicCatalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(catalog.Stations) != 1 || catalog.Stations[0].URL != "https://relay.example.com" {
		t.Fatalf("want remote link, got %+v", catalog)
	}
}

func TestPublicCatalogEmpty(t *testing.T) {
	svc := NewService(&stubRepo{})
	catalog, err := svc.PublicCatalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if catalog == nil || len(catalog.Stations) != 0 {
		t.Fatalf("want empty stations, got %+v", catalog)
	}
}
