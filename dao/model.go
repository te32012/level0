package dao

type Delivery struct {
	Name    string
	Phone   string
	Zip     string
	City    string
	Address string
	Region  string
	Email   string
}
type Item struct {
	Chrt_id      int
	Track_number string
	Price        int
	Rid          string
	Name         string
	Sale         string
	Size         string
	Total_price  int
	Nm_id        int
	Brand        string
	Staus        int
}

type Order struct {
	Order_uid          string
	Track_number       string
	Entry              string
	Delivery           *Delivery
	Items              []*Item
	Locale             string
	Internal_signature string
	Customer_id        string
	Delivery_service   string
	Shardkey           string
	Mm_id              string
	Data_created       string
	Oof_shard          string
}
