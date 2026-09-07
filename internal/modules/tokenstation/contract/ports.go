package contract

import tokenstationdomain "github.com/dujiao-next/internal/modules/tokenstation/domain"

// Repository 只存别人中转站的名称和链接。
type Repository interface {
	ListStations() ([]tokenstationdomain.Station, error)
	GetStation(id uint) (*tokenstationdomain.Station, error)
	CreateStation(station *tokenstationdomain.Station) error
	UpdateStation(station *tokenstationdomain.Station) error
	DeleteStation(id uint) error
}
