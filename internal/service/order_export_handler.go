package service

import (
	"fmt"
	"net/http"
	"partivo_tickets/internal/logic"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderExportHandler handles the HTTP request for exporting orders.
type OrderExportHandler struct {
	orderLogic logic.OrderLogic
}

// NewOrderExportHandler creates a new instance of OrderExportHandler.
func NewOrderExportHandler(ol logic.OrderLogic) *OrderExportHandler {
	return &OrderExportHandler{orderLogic: ol}
}

// ServeHTTP implements the http.Handler interface.
func (h *OrderExportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//TODO:Event Owner

	// 1. Parse parameters from the request.
	eventIDStr := r.PathValue("event_id")
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")

	if eventIDStr == "" || yearStr == "" || monthStr == "" {
		WriteHttpError(w, http.StatusBadRequest, "Missing required parameters: event_id, year, month")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		WriteHttpError(w, http.StatusBadRequest, "Invalid year format")
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		WriteHttpError(w, http.StatusBadRequest, "Invalid month format")
		return
	}

	eventID, err := primitive.ObjectIDFromHex(eventIDStr)
	if err != nil {
		WriteHttpError(w, http.StatusBadRequest, "Invalid event_id format")
		return
	}

	// 2. Call the logic layer to get the export data.
	filename, csvData, err := h.orderLogic.ExportEventOrdersByMonth(r.Context(), eventID, year, month)
	if err != nil {
		// TODO: Handle different error types (e.g., NotFound vs. Internal)
		WriteHttpError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate export: %v", err))
		return
	}

	// 3. Write the HTTP response.
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(csvData); err != nil {
		// If writing fails, it's often too late to send a different HTTP status code,
		// but we can log the error.
		// In a real app, you would use your structured logger here.
		fmt.Printf("failed to write csv data to response: %v\n", err)
	}
}
