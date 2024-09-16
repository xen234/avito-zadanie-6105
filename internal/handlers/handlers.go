package handlers

import (
	_ "database/sql"
	"encoding/json"
	"log"
	"net/http"

	"git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/api"
	"git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/internal/db"
	_ "git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/logger/sl"
)

type MyServer struct {
	Database *db.DB
}

var _ api.ServerInterface = (*MyServer)(nil)

func NewServer(storage *db.DB) *MyServer {
	return &MyServer{
		Database: storage,
	}
}

// Проверка доступности сервера
// (GET /ping)
func (s *MyServer) CheckServer(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Получение списка тендеров
// (GET /tenders)
func (s *MyServer) GetTenders(w http.ResponseWriter, r *http.Request, params api.GetTendersParams) {
	if params.Limit != nil && *params.Limit < 0 {
		http.Error(w, `{"error": "invalid limit parameter"}`, http.StatusBadRequest)
		return
	}

	if params.Offset != nil && *params.Offset < 0 {
		http.Error(w, `{"error": "invalid offset parameter"}`, http.StatusBadRequest)
		return
	}

	tenders, err := s.Database.GetTenders(params)
	if err != nil {
		log.Printf("Error fetching tenders: %v", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tenders)
}

// Получить тендеры пользователя
// (GET /tenders/my)
func (s *MyServer) GetUserTenders(w http.ResponseWriter, r *http.Request, params api.GetUserTendersParams) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, `{"error": "username is required"}`, http.StatusUnauthorized)
		return
	}

	var limit, offset int32
	if params.Limit != nil {
		limit = *params.Limit
	} else {
		limit = 10
	}

	if params.Offset != nil {
		offset = *params.Offset
	} else {
		offset = 0
	}

	tenders, err := s.Database.GetUserTenders(username, limit, offset)
	if err != nil {
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tenders)
}

// Создание нового тендера
// (POST /tenders/new)

type CreateTenderRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	ServiceType     string `json:"serviceType"`
	OrganizationId  string `json:"organizationId"`
	CreatorUsername string `json:"creatorUsername"`
}

func (s *MyServer) CreateTender(w http.ResponseWriter, r *http.Request) {
	var request CreateTenderRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if request.Name == "" || request.Description == "" || request.ServiceType == "" || request.OrganizationId == "" || request.CreatorUsername == "" {
		log.Println("Error: missing required fields in request body")
		http.Error(w, `{"error": "missing required fields"}`, http.StatusBadRequest)
		return
	}

	serviceType := api.TenderServiceType(request.ServiceType)
	newTender := api.Tender{
		Name:           request.Name,
		Description:    request.Description,
		ServiceType:    serviceType,
		OrganizationId: request.OrganizationId,
		Status:         "CREATED",
		Version:        1,
	}

	log.Printf("Creating tender: %v", request.CreatorUsername)
	createdTender, err := s.Database.CreateTender(newTender, request.CreatorUsername)
	if err != nil {
		log.Printf("Error creating tender: %v", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("Tender created successfully: %v", createdTender)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(createdTender)
}

// Редактирование тендера
// (PATCH /tenders/{tenderId}/edit)
func (s *MyServer) EditTender(w http.ResponseWriter, r *http.Request, tenderId api.TenderId, params api.EditTenderParams) {
	if tenderId == "" || params.Username == "" {
		http.Error(w, `{"error": "tenderId and username are required"}`, http.StatusBadRequest)
		return
	}

	var updates struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ServiceType string `json:"serviceType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	updatedTender, err := s.Database.EditTender(tenderId, updates.Name, updates.Description, updates.ServiceType, params.Username)
	if err != nil {
		if err == db.ErrForbidden {
			http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
		} else if err == db.ErrTenderNotFound {
			http.Error(w, `{"error": "tender not found"}`, http.StatusNotFound)
		} else if err == db.ErrUserNotFound {
			http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedTender); err != nil {
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Откат версии тендера
// (PUT /tenders/{tenderId}/rollback/{version})
func (s *MyServer) RollbackTender(w http.ResponseWriter, r *http.Request, tenderId api.TenderId, version int32, params api.RollbackTenderParams) {
	if tenderId == "" || version < 1 || params.Username == "" {
		http.Error(w, `{"error": "tenderId, version, and username are required"}`, http.StatusBadRequest)
		return
	}

	updatedTender, err := s.Database.RollbackTender(string(tenderId), int(version), params.Username)
	if err != nil {
		switch err {
		case db.ErrForbidden:
			http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
		case db.ErrTenderNotFound:
			http.Error(w, `{"error": "tender not found"}`, http.StatusNotFound)
		case db.ErrUserNotFound:
			http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
		default:
			http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedTender); err != nil {
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Получение текущего статуса тендера
// (GET /tenders/{tenderId}/status)
func (s *MyServer) GetTenderStatus(w http.ResponseWriter, r *http.Request, tenderId api.TenderId, params api.GetTenderStatusParams) {
	if tenderId == "" || *params.Username == "" {
		http.Error(w, `{"error": "tenderId and username are required"}`, http.StatusBadRequest)
		return
	}

	status, err := s.Database.GetTenderStatus(tenderId, *params.Username)
	if err != nil {
		if err == db.ErrForbidden {
			http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
		} else if err == db.ErrTenderNotFound {
			http.Error(w, `{"error": "tender not found"}`, http.StatusNotFound)
		} else if err == db.ErrUserNotFound {
			http.Error(w, `{"error": "user not found"}`, http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Изменение статуса тендера
// (PUT /tenders/{tenderId}/status)
func (s *MyServer) UpdateTenderStatus(w http.ResponseWriter, r *http.Request, tenderId api.TenderId, params api.UpdateTenderStatusParams) {
	if tenderId == "" || params.Status == "" || params.Username == "" {
		http.Error(w, `{"error": "tenderId, status, and username are required"}`, http.StatusBadRequest)
		return
	}

	updatedTender, err := s.Database.UpdateTenderStatus(tenderId, params.Status, params.Username)
	if err != nil {
		if err == db.ErrForbidden {
			http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
		} else if err == db.ErrTenderNotFound {
			http.Error(w, `{"error": "tender not found"}`, http.StatusNotFound)
		} else if err == db.ErrUserNotFound {
			http.Error(w, `{"error": "user not found"}`, http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedTender); err != nil {
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Получение списка ваших предложений
// (GET /bids/my)
func (s *MyServer) GetUserBids(w http.ResponseWriter, r *http.Request, params api.GetUserBidsParams) {
	if *params.Username == "" {
		http.Error(w, `{"error": "username is required"}`, http.StatusBadRequest)
		return
	}

	var limit, offset int32
	if params.Limit != nil {
		limit = *params.Limit
	} else {
		limit = 10
	}

	if params.Offset != nil {
		offset = *params.Offset
	} else {
		offset = 0
	}

	bids, err := s.Database.GetUserBids(limit, offset, *params.Username)
	if err != nil {
		if err == db.ErrForbidden {
			http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
		} else if err == db.ErrUserNotFound {
			http.Error(w, `{"error": "user not found"}`, http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	response, err := json.Marshal(bids)
	if err != nil {
		http.Error(w, `{"error": "error marshalling response"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

// Создание нового предложения
// (POST /bids/new)
type CreateBidRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TenderId    string `json:"tenderId"`
	AuthorId    string `json:"authorId"`
	AuthorType  string `json:"authorType"`
}

func (s *MyServer) CreateBid(w http.ResponseWriter, r *http.Request) {
	var request CreateBidRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if request.Name == "" || request.Description == "" || request.TenderId == "" || request.AuthorId == "" || request.AuthorType == "" {
		log.Println("Error: missing required fields in request body")
		http.Error(w, `{"error": "missing required fields"}`, http.StatusBadRequest)
		return
	}

	authorType := api.BidAuthorType(request.AuthorType)
	if authorType != "USER" && authorType != "ORGANIZATION" {
		http.Error(w, `{"error": "invalid author type"}`, http.StatusBadRequest)
		return
	}

	newBid := api.Bid{
		Name:        request.Name,
		Description: request.Description,
		TenderId:    api.TenderId(request.TenderId),
		AuthorId:    api.BidAuthorId(request.AuthorId),
		AuthorType:  authorType,
		Status:      "CREATED",
		Version:     1,
	}

	log.Printf("Creating bid: %v", request.AuthorId)
	createdBid, err := s.Database.CreateBid(newBid)
	if err != nil {
		log.Printf("Error creating bid: %v", err)
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("Bid created successfully: %v", createdBid)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(createdBid)
}

// Редактирование параметров предложения
// (PATCH /bids/{bidId}/edit)
func (s *MyServer) EditBid(w http.ResponseWriter, r *http.Request, bidId api.BidId, params api.EditBidParams) {
	w.WriteHeader(http.StatusOK)
}

// Отправка отзыва по предложению
// (PUT /bids/{bidId}/feedback)
func (s *MyServer) SubmitBidFeedback(w http.ResponseWriter, r *http.Request, bidId api.BidId, params api.SubmitBidFeedbackParams) {
	w.WriteHeader(http.StatusOK)
}

// Откат версии предложения
// (PUT /bids/{bidId}/rollback/{version})
func (s *MyServer) RollbackBid(w http.ResponseWriter, r *http.Request, bidId api.BidId, version int32, params api.RollbackBidParams) {
	w.WriteHeader(http.StatusOK)
}

// Получение текущего статуса предложения
// (GET /bids/{bidId}/status)
func (s *MyServer) GetBidStatus(w http.ResponseWriter, r *http.Request, bidId api.BidId, params api.GetBidStatusParams) {
	w.WriteHeader(http.StatusOK)
}

// Изменение статуса предложения
// (PUT /bids/{bidId}/status)
func (s *MyServer) UpdateBidStatus(w http.ResponseWriter, r *http.Request, bidId api.BidId, params api.UpdateBidStatusParams) {
	w.WriteHeader(http.StatusOK)
}

// Отправка решения по предложению
// (PUT /bids/{bidId}/submit_decision)
func (s *MyServer) SubmitBidDecision(w http.ResponseWriter, r *http.Request, bidId api.BidId, params api.SubmitBidDecisionParams) {
	w.WriteHeader(http.StatusOK)
}

// Получение списка предложений для тендера
// (GET /bids/{tenderId}/list)
func (s *MyServer) GetBidsForTender(w http.ResponseWriter, r *http.Request, tenderId api.TenderId, params api.GetBidsForTenderParams) {
	w.WriteHeader(http.StatusOK)
}

// Просмотр отзывов на прошлые предложения
// (GET /bids/{tenderId}/reviews)
func (s *MyServer) GetBidReviews(w http.ResponseWriter, r *http.Request, tenderId api.TenderId, params api.GetBidReviewsParams) {
	w.WriteHeader(http.StatusOK)
}
