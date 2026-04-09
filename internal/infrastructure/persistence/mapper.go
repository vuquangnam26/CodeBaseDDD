package persistence

import (
	"github.com/namcuongq/order-service/internal/domain/order"
)

// --- Domain -> Persistence ---

func ToOrderModel(o *order.Order) OrderModel {
	return OrderModel{
		ID:          o.ID().UUID(),
		CustomerID:  o.CustomerID().UUID(),
		Status:      o.Status().String(),
		TotalAmount: o.TotalAmount().Amount(),
		Version:     o.Version(),
		CreatedAt:   o.CreatedAt(),
		UpdatedAt:   o.UpdatedAt(),
	}
}

func ToOrderItemModels(o *order.Order) []OrderItemModel {
	items := o.Items()
	models := make([]OrderItemModel, 0, len(items))
	for _, item := range items {
		models = append(models, OrderItemModel{
			ID:          item.ID(),
			OrderID:     o.ID().UUID(),
			ProductID:   item.ProductID(),
			ProductName: item.ProductName(),
			Quantity:    item.Quantity(),
			UnitPrice:   item.UnitPrice().Amount(),
		})
	}
	return models
}

// --- Persistence -> Domain ---

func ToDomainOrder(m OrderModel, itemModels []OrderItemModel) *order.Order {
	items := make([]order.OrderItem, 0, len(itemModels))
	for _, im := range itemModels {
		items = append(items, order.ReconstructOrderItem(
			im.ID,
			im.ProductID,
			im.ProductName,
			im.Quantity,
			im.UnitPrice,
		))
	}

	return order.ReconstructOrder(
		m.ID,
		m.CustomerID,
		m.Status,
		m.Version,
		m.CreatedAt,
		m.UpdatedAt,
		items,
	)
}
