package database

type Task struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Service     string `json:"service"`
	Time        string `json:"time"`
	DaysOfWeek  string `json:"days_of_week"`
	IsRecurring bool   `json:"is_recurring"`
	Description string `json:"description,omitempty"`
}
