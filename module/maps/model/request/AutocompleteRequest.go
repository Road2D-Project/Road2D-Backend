package request

type AutocompleteRequest struct {
	Input        string `form:"input" binding:"required"`
	Location     string `form:"location"`
	SessionToken string `form:"sessiontoken"`
	Limit        int    `form:"limit"`
}
