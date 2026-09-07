package application

import (
	"net/url"
	"strings"
	"time"

	tokenstationcontract "github.com/dujiao-next/internal/modules/tokenstation/contract"
	tokenstationdomain "github.com/dujiao-next/internal/modules/tokenstation/domain"
)

// Service 管理别人中转站的网页链接。
type Service struct {
	repo tokenstationcontract.Repository
}

func NewService(repo tokenstationcontract.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListStations() ([]tokenstationdomain.Station, error) {
	return s.repo.ListStations()
}

func (s *Service) CreateStation(input tokenstationcontract.StationInput) (*tokenstationdomain.Station, error) {
	baseURL, name, err := normalizeStation(input)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	station := &tokenstationdomain.Station{
		Name:      name,
		BaseURL:   baseURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateStation(station); err != nil {
		return nil, err
	}
	return station, nil
}

func (s *Service) UpdateStation(id uint, input tokenstationcontract.StationInput) (*tokenstationdomain.Station, error) {
	station, err := s.requireStation(id)
	if err != nil {
		return nil, err
	}
	baseURL, name, err := normalizeStation(input)
	if err != nil {
		return nil, err
	}
	station.Name = name
	station.BaseURL = baseURL
	station.UpdatedAt = time.Now()
	if err := s.repo.UpdateStation(station); err != nil {
		return nil, err
	}
	return station, nil
}

func (s *Service) DeleteStation(id uint) error {
	if id == 0 {
		return tokenstationcontract.ErrStationNotFound
	}
	return s.repo.DeleteStation(id)
}

func (s *Service) PublicCatalog() (*tokenstationcontract.Catalog, error) {
	items, err := s.repo.ListStations()
	if err != nil {
		return nil, err
	}
	out := &tokenstationcontract.Catalog{Stations: []tokenstationcontract.PublicStation{}}
	for _, station := range items {
		if strings.TrimSpace(station.BaseURL) == "" {
			return nil, tokenstationcontract.ErrStationInvalid
		}
		out.Stations = append(out.Stations, tokenstationcontract.PublicStation{
			Name: station.Name,
			URL:  station.BaseURL,
		})
	}
	return out, nil
}

func (s *Service) requireStation(id uint) (*tokenstationdomain.Station, error) {
	if id == 0 {
		return nil, tokenstationcontract.ErrStationNotFound
	}
	station, err := s.repo.GetStation(id)
	if err != nil {
		return nil, err
	}
	if station == nil {
		return nil, tokenstationcontract.ErrStationNotFound
	}
	return station, nil
}

func normalizeStation(input tokenstationcontract.StationInput) (string, string, error) {
	name := strings.TrimSpace(input.Name)
	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if name == "" || baseURL == "" {
		return "", "", tokenstationcontract.ErrStationInvalid
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", tokenstationcontract.ErrStationInvalid
	}
	return baseURL, name, nil
}
