package response

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginEnvelope struct {
	Data LoginResponse `json:"data"`
}
