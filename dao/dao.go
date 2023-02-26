package dao

import (
	"context"
	"fmt"
	"os"

	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

// Создаем соединение к базе данных, создаем таблицы в базе данных
func (base *Database) Conect(user string, password string, host string, port string, database string) {

	url_database := "postgres://" + user + ":" + password + "@" + host + ":" + port + "/" + database
	configuration, err := pgxpool.ParseConfig(url_database)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка настройки конфигурации базы данных: %v\n", err)
		os.Exit(1)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), configuration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка соединения к базе: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stderr, "Подключились к базе данных по адресу : %v \n", url_database)
	}

	create_table_delivery := `create table if not exists delivery (
		Name    varchar(256),
		Phone   varchar(256) primary key not null,
		Zip     varchar(256),
		City    varchar(256),
		Address varchar(256),
		Region  varchar(256),
		Email   varchar(256)
		);`

	create_table_items := `create table if not exists items (
		Chrt_id      integer primary key not null,
		Track_number varchar(256),
		Price        integer,
		Rid          varchar(256),
		Name         varchar(256),
		Sale         varchar(256),
		Size         varchar(256),
		Total_price  integer,
		Nm_id        integer,
		Brand        varchar(256),
		Staus        integer
		);`

	create_table_order := `create table if not exists orders  (
		Order_uid varchar(256) primary key not null,
		Track_number varchar(256), 
		Entry varchar(256),
		Delivery varchar(256) references Delivery (Phone), 
		Locale varchar(256),
		Internal_signature varchar(256), 
		Customer_id varchar(256), 
		Delivery_service varchar(256),
		Shardkey varchar(256),
		Mm_id varchar(256), 
		Data_created varchar(256), 
		Oof_shard varchar(256)
		);`

	create_table_itemsorder := `create table if not exists itemorder (
		Order_uid varchar(256) references orders (Order_uid),
		Chrt_id integer references items (Chrt_id)
	);`
	transation, err := pool.Begin(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания транзакции (транзакция создающая таблицы в базе данных): %v\n", err)
		transation.Rollback(context.Background())
		os.Exit(1)
	}

	transation.Exec(context.Background(), create_table_delivery)

	transation.Exec(context.Background(), create_table_items)

	transation.Exec(context.Background(), create_table_order)

	transation.Exec(context.Background(), create_table_itemsorder)

	transation.Commit(context.Background())

	base.Pool = pool
}

// Получаем из базы по id заказ
// Использовать если не нашли в кеше данные
func (base *Database) GetOrderById(order_id string) (Order, error) {

	var sql_orders string = `select 
		Order_uid, 
		Track_number, 
		Entry, 
		Delivery, 
		Locale, 
		Internal_signature, 
		Customer_id, 
		Delivery_service, 
		Shardkey, 
		Mm_id, 
		Data_created, 
		Oof_shard from orders where Order_uid in ($1) ;`

	var sql_delivery string = `select Name, Phone, Zip, City, Address, Region, Email from delivery where Phone in ($1) ;`

	var sql_itemsorder string = `select Order_uid, Chrt_id from itemorder where Order_uid in ($1) ;`

	var sql_items string = `select Chrt_id, Track_number, Price, Rid, Name, Sale, Size, Total_price, Nm_id, Brand, Staus from items where Chrt_id in ($1);`

	transaction, err := base.Pool.Begin(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания транзакции select : %v\n", err)
		return Order{}, err
	}

	row := transaction.QueryRow(context.Background(), sql_orders, order_id)

	var delivery Delivery
	var order Order

	er := row.Scan(&order.Order_uid, &order.Track_number, &order.Entry, &delivery.Phone,
		&order.Locale, &order.Internal_signature, &order.Customer_id,
		&order.Delivery_service, &order.Shardkey, &order.Mm_id,
		&order.Data_created, &order.Oof_shard)
	if er != nil {
		fmt.Fprintf(os.Stderr, "Ошибка get order базы данных: %v\n", er)
		return Order{}, er
	} else {
		fmt.Fprintf(os.Stderr, "Достали order (без вложений - items, delivery) из базы\n")
	}

	var items []*Item

	fmt.Println("select from itemorder where Order_uid = " + order_id)
	rows, err := transaction.Query(context.Background(), sql_itemsorder, order.Order_uid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка get items базы данных: %v\n", err)
		return Order{}, err
	} else {
		fmt.Fprintf(os.Stderr, "Достали список items (соответсвующий order) из базы\n")

	}

	var itemorder map[int]string = make(map[int]string)

	// Если не создавать map - то возникает ошибка conn busy (баг библиотеки скорее всего транзакция блокирует чтение секции в бд где лежат данные)
	// Из-за бага последующее чтение становится не возможным
	var chrt_id int
	var ord_id string
	for rows.Next() {
		rows.Scan(&ord_id, &chrt_id)
		itemorder[chrt_id] = ord_id
	}

	for chrt_id, ord_id := range itemorder {
		fmt.Println("chrt_id = " + strconv.Itoa(chrt_id) + " ord_id = " + ord_id)
		fmt.Println("select from items where chrt_id = " + strconv.Itoa(chrt_id))
		rows := transaction.QueryRow(context.Background(), sql_items, chrt_id)
		var item Item
		err := rows.Scan(&item.Chrt_id, &item.Track_number, &item.Price, &item.Rid, &item.Name, &item.Sale, &item.Size, &item.Total_price, &item.Nm_id, &item.Brand, &item.Staus)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка get item базы данных: %v\n", err)
			return Order{}, err
		} else {
			fmt.Fprintf(os.Stderr, "Успешно загрузили item c id : %v из базы\n", strconv.Itoa(item.Chrt_id))
		}
		items = append(items, &item)
	}
	order.Items = items

	fmt.Println("select from delivery where phone = " + delivery.Phone)
	row1 := transaction.QueryRow(context.Background(), sql_delivery, delivery.Phone)
	e := row1.Scan(&delivery.Name, &delivery.Phone, &delivery.Zip, &delivery.City, &delivery.Address, &delivery.Region, &delivery.Email)

	if e != nil {
		fmt.Fprintf(os.Stderr, "Ошибка get delivery базы данных: %v\n", e)
		return Order{}, err
	} else {
		fmt.Fprintf(os.Stderr, "Успешно загрузили delivery c id : %v из базы\n", delivery.Phone)

	}
	order.Delivery = &delivery
	return order, nil
}

func (base *Database) InsertOrder(order Order) {
	var sql_orders string = `insert into orders 
		(Order_uid, 
		Track_number, 
		Entry, 
		Delivery, 
		Locale, 
		Internal_signature, 
		Customer_id, 
		Delivery_service, 
		Shardkey, 
		Mm_id, 
		Data_created, 
		Oof_shard ) values 
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`

	var sql_delivery string = `insert into delivery 
	(Name, 
	Phone, 
	Zip, 
	City, 
	Address, 
	Region, 
	Email) values
	($1, $2, $3, $4, $5, $6, $7);`

	var sql_items string = `insert into items 
		(Chrt_id, 
		Track_number, 
		Price, 
		Rid, 
		Name, 
		Sale, 
		Size, 
		Total_price, 
		Nm_id, 
		Brand, 
		Staus) values 
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`

	var sql_itemsorder string = `insert into itemorder (Order_uid, Chrt_id) values ($1, $2)`

	transaction, err := base.Pool.Begin(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания транзакции insert: %v\n", err)
		os.Exit(1)
	}

	transaction.Exec(context.Background(), sql_delivery,
		order.Delivery.Name,
		order.Delivery.Phone,
		order.Delivery.Zip,
		order.Delivery.City,
		order.Delivery.Address,
		order.Delivery.Region,
		order.Delivery.Email)

	transaction.Exec(context.Background(), sql_orders,
		order.Order_uid, order.Track_number, order.Entry,
		order.Delivery.Phone, order.Locale, order.Internal_signature,
		order.Customer_id, order.Delivery_service, order.Shardkey,
		order.Mm_id, order.Data_created, order.Oof_shard)

	for _, value := range order.Items {
		transaction.Exec(context.Background(), sql_items,
			value.Chrt_id,
			value.Track_number,
			value.Price,
			value.Rid,
			value.Name,
			value.Sale,
			value.Size,
			value.Total_price,
			value.Nm_id,
			value.Brand,
			value.Staus)
		transaction.Exec(context.Background(), sql_itemsorder, order.Order_uid, value.Chrt_id)
	}
	transaction.Commit(context.Background())

}
