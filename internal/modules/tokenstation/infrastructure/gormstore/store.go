package gormstore

import (
	tokenstationcontract "github.com/dujiao-next/internal/modules/tokenstation/contract"
	tokenstationdomain "github.com/dujiao-next/internal/modules/tokenstation/domain"

	"gorm.io/gorm"
)

type Store struct{ db *gorm.DB }

var _ tokenstationcontract.Repository = (*Store)(nil)

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) ListStations() ([]tokenstationdomain.Station, error) {
	var items []tokenstationdomain.Station
	if err := s.db.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetStation(id uint) (*tokenstationdomain.Station, error) {
	if id == 0 {
		return nil, nil
	}
	var item tokenstationdomain.Station
	err := s.db.First(&item, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) CreateStation(station *tokenstationdomain.Station) error {
	return s.db.Create(station).Error
}

func (s *Store) UpdateStation(station *tokenstationdomain.Station) error {
	return s.db.Save(station).Error
}

func (s *Store) DeleteStation(id uint) error {
	return s.db.Delete(&tokenstationdomain.Station{}, id).Error
}
