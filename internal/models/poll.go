package models

type Poll struct {
	ID           int    `json:"id"`
	PollType     string `json:"poll_type"`
	TargetUserID int    `json:"target_user_id"`
	InitiatorID  int    `json:"initiator_id"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
}
type PollVote struct {
	ID      int  `json:"id"`
	PollID  int  `json:"poll_id"`
	VoterID int  `json:"voter_id"`
	Vote    bool `json:"vote"`
}
