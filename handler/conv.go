package handler

import (
	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/gen/pb"
)

func ToSearchFields(pb *pb.SearchFields) *entity.SearchFields {
	if pb == nil {
		return &entity.SearchFields{}
	}

	return &entity.SearchFields{
		DoubleArgs: pb.GetDoubleArgs(),
		StringArgs: pb.GetStringArgs(),
		Tags:       pb.GetTags(),
	}
}
