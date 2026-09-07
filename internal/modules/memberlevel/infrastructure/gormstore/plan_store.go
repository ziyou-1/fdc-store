package gormstore

import (
	"errors"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	"gorm.io/gorm"
)

type PlanStore struct {
	db *gorm.DB
}

func NewPlanStore(db *gorm.DB) *PlanStore {
	return &PlanStore{db: db}
}

func (r *PlanStore) GetByPeriod(period string) (*memberleveldomain.Plan, error) {
	var plan memberleveldomain.Plan
	if err := r.db.Where("period = ?", period).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *PlanStore) List() ([]memberleveldomain.Plan, error) {
	var plans []memberleveldomain.Plan
	if err := r.db.Order("period asc").Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *PlanStore) Create(plan *memberleveldomain.Plan) error {
	return r.db.Create(plan).Error
}

func (r *PlanStore) Update(plan *memberleveldomain.Plan) error {
	if plan == nil || plan.Period == "" {
		return nil
	}
	return r.db.Model(&memberleveldomain.Plan{}).
		Where("period = ?", plan.Period).
		Select("price", "name_json", "description_json", "is_active", "updated_at").
		Updates(plan).Error
}

var _ memberlevelcontract.PlanRepository = (*PlanStore)(nil)
