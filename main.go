package main

import (
	"encoding/json"
	"fmt"
	cash "generatorjson/cash"
	dao "generatorjson/dao"
	"math/rand"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"os"

	"github.com/gin-gonic/gin"
	stan "github.com/nats-io/stan.go"
)

var n = 30
var t = rand.NewSource(time.Now().UnixNano())

func GenerateConfig() {
	/* Переменные базы данных
	 */
	os.Setenv("db_username", "user")
	os.Setenv("db_password", "user")
	os.Setenv("db_hostname", "db")
	os.Setenv("db_port", "5432")
	os.Setenv("db_dbname", "database")

	/* Переменные брокера
	 */
	os.Setenv("stanClusterID", "NATS")
	os.Setenv("clientID", "client")
	/* Переменные кеширования
	 */
}

type dataConnector struct {
	NameSubject    string
	Base           *dao.Database
	Cash           *cash.Cash
	StanConnection *stan.Conn
}

// Конструктор конектора
func (connector *dataConnector) dataConnector() {
	var cash cash.Cash = cash.Cash{}
	cash.Cash("/go/src/app/cashdirectory/cash.bin", 2)
	connector.Cash = &cash
	var base dao.Database = dao.Database{}
	base.Conect(os.Getenv("db_username"), os.Getenv("db_password"), os.Getenv("db_hostname"), os.Getenv("db_port"), os.Getenv("db_dbname"))
	connector.Base = &base
	StanConnection, err := stan.Connect(os.Getenv("stanClusterID"), os.Getenv("clientID"), stan.NatsURL("nats://nts:4222"), stan.Pings(10, 5))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка присоединения клиента %v к кластеру %v  %v \n", os.Getenv("clientID"), os.Getenv("stanClusterID"), err)
		return
	} else {
		fmt.Fprintf(os.Stderr, "Клиент %v подключился к кластеру %v \n", os.Getenv("clientID"), os.Getenv("stanClusterID"))

	}
	connector.StanConnection = &StanConnection
	connector.NameSubject = "canal"
}

// В этом методе при помощи брокера отправляем подписчикам данные заказов
func (connector *dataConnector) publishOrder(data dao.Order) {
	b, e := json.Marshal(data)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Ошибка (сериализации) сообщения %v \n", e)
		return
	}
	// Обработчик ошибок публикации (функция вызываемая при ошибках в публикации)
	ackHandler := func(ackedNuid string, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка публикации сообщения id %s: %v\n", ackedNuid, err)
		} else {
			fmt.Fprintf(os.Stderr, "Опубликовали сообщение id  %v \n", ackedNuid)
		}
	}
	nuid, err := (*connector.StanConnection).PublishAsync(connector.NameSubject, b, ackHandler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка публикации сообщения id %s: %v\n", nuid, err)
	}
}

// В этом методе добавляем подписчика с указанным в параметре nameOfSubscriber именем
// Данные кешируются при получении
func (connector *dataConnector) subscribOnSubjectWithName(nameOfSubscriber string) {
	_, er := (*connector.StanConnection).Subscribe(connector.NameSubject, func(m *stan.Msg) {
		var order dao.Order
		e := json.Unmarshal(m.Data, &order)
		if e != nil {
			fmt.Fprintf(os.Stderr, "Ошибка полученеия (десериализации) сообщения %v \n", e)
			return
		}
		fmt.Fprintf(os.Stderr, "Получено сообщение: %s\n", order.Order_uid)
		connector.Base.InsertOrder(order)
		fmt.Fprintf(os.Stderr, "Успешно загрузили order с id: %s в базу\n", order.Order_uid)

		connector.Cash.SetCashInMemory(order)
		fmt.Fprintf(os.Stderr, "Успешно загрузили order с id: %s в кеш\n", order.Order_uid)

		connector.Cash.SaveCashToFile()
		fmt.Fprintf(os.Stderr, "Сохранили кеш в контейнере \n")

		err := m.Ack()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Не можем подтвердить получение сообщения - ошибка типа %v \n", err)
			return

		}
	}, stan.DurableName(nameOfSubscriber), stan.SetManualAckMode())
	if er != nil {
		fmt.Fprintf(os.Stderr, "Ошибка подписки %v типа %v  \n", nameOfSubscriber, er)

	}
}

// Запускаем прослушивание localhost:8080/orders/:id
// где :id номер заказа
// если такого заказа нет то 404
func (conector *dataConnector) startserver() {
	route := gin.Default()
	route.StaticFile("/order", "./web/index.html")
	route.GET("/orders/:id", func(c *gin.Context) {
		id := c.Param("id")
		var order_from_db dao.Order
		var error_from_db error
		order_from_cash, error_cash := conector.Cash.GetCashFromMemory(id)
		if error_cash != nil {
			fmt.Fprintf(os.Stderr, "Ошибка поиска в кеше ID = %v сгенерированна ошибка %v  \n", id, error_cash)
			order_from_db, error_from_db = conector.Base.GetOrderById(id)
			if error_from_db != nil {
				c.JSON(404, "Ошибка поиска id = "+id+"в базе \n")
			} else {
				conector.Cash.SetCashInMemory(order_from_db)
				conector.Cash.SaveCashToFile()
				c.JSON(http.StatusOK, order_from_db)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Данные с id order: %v были загружены из кеша \n", id)
			c.JSON(http.StatusOK, order_from_cash)
		}
	})
	route.Run()
}
func (connector *dataConnector) close() {
	connector.Base.Pool.Close()
	(*connector.StanConnection).Close()
}

func main() {

	GenerateConfig()

	//Создаем тестовые данные//

	var o1 dao.Order = getOrder()
	fmt.Println("Order_uid = " + o1.Order_uid)
	fmt.Println("Phone = " + o1.Delivery.Phone)
	for _, value := range o1.Items {
		fmt.Println("Chrt_id =" + strconv.Itoa(value.Chrt_id))
	}
	var o2 dao.Order = getOrder()
	fmt.Println("Order_uid = " + o2.Order_uid)
	fmt.Println("Phone = " + o2.Delivery.Phone)
	for _, value := range o2.Items {
		fmt.Println("Chrt_id =" + strconv.Itoa(value.Chrt_id))
	}
	var o3 dao.Order = getOrder()
	fmt.Println("Order_uid = " + o3.Order_uid)
	fmt.Println("Phone = " + o3.Delivery.Phone)
	for _, value := range o3.Items {
		fmt.Println("Chrt_id =" + strconv.Itoa(value.Chrt_id))
	}

	var connector dataConnector
	connector.dataConnector()
	connector.subscribOnSubjectWithName("nameOfSubscribshion")
	connector.publishOrder(o1)
	connector.publishOrder(o2)
	connector.publishOrder(o3)
	connector.startserver()
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT)
	<-s
	connector.close()
}

func getOrder() dao.Order {
	rand.Seed(time.Now().UnixNano())
	var js dao.Order = dao.Order{getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20)),
		getDelivery(), getItems(), getString(rand.Intn(20)), getString(rand.Intn(20)),
		getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20)),
		getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20))}
	return js
}

func getItems() []*dao.Item {
	rand.Seed(time.Now().UnixNano())
	var items []*dao.Item
	for x := 0; x < rand.Intn(20); x++ {
		items = append(items, getItem())
	}
	return items
}

func getItem() *dao.Item {
	rand.Seed(time.Now().UnixNano())
	var d *dao.Item = &dao.Item{rand.Intn(1000), getString(rand.Intn(20)), rand.Intn(1000), getString(rand.Intn(20)),
		getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20)), rand.Intn(1000), rand.Intn(1000), getString(rand.Intn(20)), rand.Intn(1000)}
	return d
}

func getDelivery() *dao.Delivery {
	rand.Seed(time.Now().UnixNano())
	var d *dao.Delivery = &dao.Delivery{getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20)),
		getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20)), getString(rand.Intn(20))}
	return d
}

func getString(n int) string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := n
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	str := b.String()
	return str
}
