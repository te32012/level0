package cash

import (
	"encoding/json"
	"errors"
	"fmt"
	dao "generatorjson/dao"
	"os"
)

type Cash struct {
	// Имя файла в который кешируем
	// Желательно указывать постоянный источник не зависящий от перезапуска контейнера
	// В таком случае при перезапуске контейнеров данные кешируются на этом томе
	Name_file string
	// Размер кеша (элементов)
	Size int
	// Сам кеш - ключ значение (достаем элементы за О(1))
	Data map[string]dao.Order
	// Держим порядок загрузки в кеш
	Queue []string
	// После какого элемента вставляем (если n = Size то вставляем на первый), после вставки увеличиваем на 1 n
	Number int
}

func (cash *Cash) Cash(file_name string, sizeCash int) {
	cash.Name_file = file_name
	cash.Size = sizeCash
	cash.Data = make(map[string]dao.Order)
	cash.Queue = make([]string, cash.Size)
	cash.Number = 0
}

// Получаем данные из оперативной памяти
func (cash *Cash) GetCashFromMemory(order_uid string) (dao.Order, error) {
	_, ok := cash.Data[order_uid]
	if ok {
		return cash.Data[order_uid], nil
	} else {
		return dao.Order{}, errors.New("по такому id кеша нет : " + order_uid)
	}
}

// Сохраняем данные в оперативную память
func (cash *Cash) SetCashInMemory(order dao.Order) {
	if cash.Number >= cash.Size {
		cash.Number = 0
		delete(cash.Data, cash.Queue[cash.Number])
		cash.Data[order.Order_uid] = order
		cash.Queue[cash.Number] = order.Order_uid
	} else {
		cash.Data[order.Order_uid] = order
		cash.Queue[cash.Number] = order.Order_uid
		cash.Number = cash.Number + 1
	}
}

// Сохраняем данные в файл расположенный на примонтированном к докер контейнеру томе
func (cash *Cash) SaveCashToFile() {
	file, err := os.OpenFile(cash.Name_file, os.O_WRONLY|os.O_CREATE, 0o777)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка открытия кеш-файла: %v\n", err)
		os.Exit(1)
	}
	b, err := json.Marshal(cash.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка серилизации данных: %v\n", err)
		os.Exit(1)
	}
	_, e := file.Write(b)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Ошибка записи данных в кеш-файл: %v\n", e)
		os.Exit(1)
	}
}

// Скачиваем данные из примонтированного докер тома
func (cash *Cash) DownloadCashFromFile() {
	file, err := os.OpenFile(cash.Name_file, os.O_RDONLY, 0o777)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка открытия кеш-файла: %v\n", err)
		os.Exit(1)
	}
	v, _ := os.Stat(file.Name())

	var b []byte = make([]byte, v.Size())
	n, e := file.Read(b)
	fmt.Println(n)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Ошибка чтения данных из кеш-файла: %v\n", e)
		os.Exit(1)
	}

	// игнорим ошибки десериализации из-за того, что у функции ложные срабатывания
	// var m map[string]interface{}
	var m map[string]dao.Order
	er := json.Unmarshal(b, &m)
	if er != nil {
		fmt.Fprintf(os.Stderr, "Ошибка десериализации прочитанных данных из кеш-файла: %v\n", er)
		os.Exit(1)
	} else {
		cash.Data = m
	}
	fmt.Println(cash.Data)

	cash.Number = 0
	for k := range cash.Data {
		if cash.Number < cash.Size {
			cash.Queue[cash.Number] = k
			cash.Number++
		} else {
			cash.Number = 0
			cash.Queue[cash.Number] = k
		}
	}

}

func (cash *Cash) RemoveCashFile() {
	os.Remove(cash.Name_file)
}
