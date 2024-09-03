package models

type Order struct {
	ID                 int    `json:"id"`
	OrderNumber        string `json:"order_number"`
	PickupLocation     string `json:"pickup_location"`
	DeliveryLocation   string `json:"delivery_location"`
	PickupDate         string `json:"pickup_date"`
	DeliveryDate       string `json:"delivery_date"`
	SuggestedTruckSize string `json:"suggested_truck_size"`
	Notes              string `json:"notes"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	PickupZip          string `json:"pickup_zip"`
	DeliveryZip        string `json:"delivery_zip"`
}

type OrderLocation struct {
	ID                  int     `json:"id"`
	OrderID             int     `json:"order_id"`
	PickupLabel         string  `json:"pickup_label"`
	PickupCountryCode   string  `json:"pickup_country_code"`
	PickupCountryName   string  `json:"pickup_country_name"`
	PickupStateCode     string  `json:"pickup_state_code"`
	PickupState         string  `json:"pickup_state"`
	PickupCounty        string  `json:"pickup_county"`
	PickupCity          string  `json:"pickup_city"`
	PickupStreet        string  `json:"pickup_street"`
	PickupPostalCode    string  `json:"pickup_postal_code"`
	PickupHouseNumber   string  `json:"pickup_house_number"`
	PickupLat           float64 `json:"pickup_lat"`
	PickupLng           float64 `json:"pickup_lng"`
	DeliveryLabel       string  `json:"delivery_label"`
	DeliveryCountryCode string  `json:"delivery_country_code"`
	DeliveryCountryName string  `json:"delivery_country_name"`
	DeliveryStateCode   string  `json:"delivery_state_code"`
	DeliveryState       string  `json:"delivery_state"`
	DeliveryCounty      string  `json:"delivery_county"`
	DeliveryCity        string  `json:"delivery_city"`
	DeliveryStreet      string  `json:"delivery_street"`
	DeliveryPostalCode  string  `json:"delivery_postal_code"`
	DeliveryHouseNumber string  `json:"delivery_house_number"`
	DeliveryLat         float64 `json:"delivery_lat"`
	DeliveryLng         float64 `json:"delivery_lng"`
	EstimatedMiles      float64 `json:"estimated_miles"`
	UpdatedAt           string  `json:"updated_at"`
	CreatedAt           string  `json:"created_at"`
}

type OrderItem struct {
	ID        int     `json:"id"`
	OrderID   int     `json:"order_id"`
	Length    float64 `json:"length"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Weight    float64 `json:"weight"`
	Pieces    int     `json:"pieces"`
	Stackable bool    `json:"stackable"`
	Hazardous bool    `json:"hazardous"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type OrderEmail struct {
	ID        int    `json:"id"`
	ReplyTo   string `json:"reply_to"`
	Subject   string `json:"subject"`
	MessageID string `json:"message_id"`
	OrderID   int    `json:"order_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
