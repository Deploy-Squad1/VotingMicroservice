package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
	"votingmicroservice/internal/middlewares"
	"votingmicroservice/internal/storage"
)

type PollHandler struct {
	store *storage.Storage
}

func NewPollHandler(store *storage.Storage) *PollHandler {
	return &PollHandler{store: store}
}

func (h *PollHandler) CreateUpgradePoll(w http.ResponseWriter, r *http.Request) {
	userIDContext := r.Context().Value(middlewares.UserIDKey)
	if userIDContext == nil {
		log.Println("User ID context is missing")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := userIDContext.(int)

	currentGroup, err := h.store.GetUserGroup(userID)
	if err != nil {
		log.Println("Error getting user group: %s", err)
		http.Error(w, "User role not found", http.StatusNotFound)
		return
	}

	if currentGroup >= 3 {
		log.Println("User role is Gold, cannot upgrade further")
		http.Error(w, "You are already at the highest rank (Gold) and cannot upgrade further", http.StatusBadRequest)
		return
	}

	var requiredDuration time.Duration
	if currentGroup == 1 {
		requiredDuration = 24 * time.Hour
	} else if currentGroup == 2 {
		requiredDuration = 3 * 24 * time.Hour
	}

	dateJoined, err := h.store.GetUserDateJoined(userID)
	if err != nil {
		log.Printf("Error getting user date joined: %s", err)
		http.Error(w, "User date joined not found", http.StatusNotFound)
		return
	}

	if time.Since(dateJoined) < requiredDuration {
		log.Printf("User date joined is not enough: %s", dateJoined)
		http.Error(w, "Not enough time has passed to request this upgrade", http.StatusForbidden)
		return
	}

	hasActive, _ := h.store.HasActivePoll(userID, "upgrade")
	if hasActive {
		log.Printf("User %v alredy has active upgrade poll", userID)
		http.Error(w, "You already have an active upgrade poll", http.StatusConflict)
		return
	}

	pollID, err := h.store.CreatePoll("upgrade", userID, userID)
	if err != nil {
		log.Printf("Error creating poll: %v", err)
		http.Error(w, "Failed to create poll", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"message": "Upgrade poll started", "poll_id": pollID})
}

func (h *PollHandler) CreateKickPoll(w http.ResponseWriter, r *http.Request) {
	initiatorIDContext := r.Context().Value(middlewares.UserIDKey)
	if initiatorIDContext == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	initiatorID := initiatorIDContext.(int)

	isInquisitor, err := h.store.IsUserInquisitor(initiatorID)
	if err != nil || !isInquisitor {
		log.Printf("User %d is not an inquisitor or DB error: %v", initiatorID, err)
		http.Error(w, "Only the Inquisitor can start a kick poll", http.StatusForbidden)
		return
	}

	targetIDStr := r.PathValue("target_id")
	targetID, err := strconv.Atoi(targetIDStr)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	if initiatorID == targetID {
		log.Printf("User %d attempted to kick hisself", initiatorID)
		http.Error(w, "You cannot kick yourself", http.StatusBadRequest)
		return
	}

	hasActive, _ := h.store.HasActivePoll(targetID, "kick")
	if hasActive {
		log.Printf("Kick already active for target %d", targetID)
		http.Error(w, "A kick poll is already active for this user", http.StatusConflict)
		return
	}

	pollID, err := h.store.CreatePoll("kick", targetID, initiatorID)
	if err != nil {
		log.Printf("Failed to create kick poll for target %d: %v", targetID, err)
		http.Error(w, "Failed to create poll", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"message": "Kick poll started", "poll_id": pollID})
}

func (h *PollHandler) VoteOnPoll(w http.ResponseWriter, r *http.Request) {
	voterIDContext := r.Context().Value(middlewares.UserIDKey)
	if voterIDContext == nil {
		log.Println("Voter ID context not found")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	voterID := voterIDContext.(int)

	pollIDStr := r.PathValue("poll_id")
	pollID, err := strconv.Atoi(pollIDStr)
	if err != nil {
		http.Error(w, "Invalid poll ID", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Vote bool `json:"vote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("Failed to decode request body: %s", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	poll, err := h.store.GetPoll(pollID)
	if err != nil {
		log.Printf("DB Error retrieving poll %d: %v\n", pollID, err)
		http.Error(w, "Poll not found", http.StatusNotFound)
		return
	}

	if poll.Status != "active" {
		log.Printf("Poll status %s is not active", poll.Status)
		http.Error(w, "This poll is closed", http.StatusBadRequest)
		return
	}

	err = h.store.AddPollVote(pollID, voterID, reqBody.Vote)
	if err != nil {
		log.Printf("DB Error inserting vote: %v\n", err)
		http.Error(w, "You have already voted on this poll", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Vote recorded successfully"})
}

func (h *PollHandler) ProcessExpiredPolls(w http.ResponseWriter, r *http.Request) {
	polls, err := h.store.GetExpiredActivePolls()
	if err != nil {
		log.Println("DB Error getting expired active polls: ", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	for _, poll := range polls {
		votesFor, votesAgainst, err := h.store.CountVotes(poll.ID)
		if err != nil {
			log.Printf("DB Error counting votes for poll %d: %v\n", poll.ID, err)
		}

		isPassed := votesFor > votesAgainst

		if isPassed {
			if poll.PollType == "kick" {
				errKick := h.store.ExecuteKick(poll.TargetUserID)
				if errKick != nil {
					log.Printf("DB Error executing kick for user %d: %v\n", poll.TargetUserID, errKick)
				}
			} else if poll.PollType == "upgrade" {
				errUpgrade := h.store.ExecuteUpgrade(poll.TargetUserID)
				if errUpgrade != nil {
					log.Printf("DB Error executing upgrade for user %d: %v\n", poll.TargetUserID, errUpgrade)
				}
			}
			h.store.UpdatePollStatus(poll.ID, "passed")
		} else {
			h.store.UpdatePollStatus(poll.ID, "failed")
		}
		log.Printf("Poll %d processed. Type: %s, Target: %d, Passed: %v (For: %d, Against: %d)\n",
			poll.ID, poll.PollType, poll.TargetUserID, isPassed, votesFor, votesAgainst)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"message":         "Cron job executed successfully",
		"polls_processed": len(polls),
	})
}
