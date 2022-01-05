package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// Hour represents the datastructure returned by ffail.win
type Hour struct {
	NOKPerKWh float64   `json:"NOK_per_kWh"`
	ValidFrom time.Time `json:"valid_from"`
	ValidTo   time.Time `json:"valid_to"`
}

func getPower(date string) (*map[string]Hour, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://norway-power.ffail.win/?zone=NO1&date=%s", date), nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]Hour

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func bulk(client influxdb2.Client, org, fromDate, toDate string) error {
	start, err := time.Parse("2006-1-2", "2021-06-01")
	if err != nil {
		log.Fatal("could not parse ", err)
	}
	end, err := time.Parse("2006-1-2", "2021-06-23")
	if err != nil {
		log.Fatal(err)
	}
	//for d := start; d.Month() == start.Month(); d = d.AddDate(0, 0, 1) {
	for d := start; d.Month() >= start.Month(); d = d.AddDate(0, 0, 1) {
		if d.Month() == end.Month() && d.Day() == end.Day() {
			break
		}
		fmt.Printf("looking up spot prices for %s", d.Format("2006-01-02"))
		hours, err := getPower(d.Format("2006-01-02"))
		if err != nil {
			return err
		}
		var points []*write.Point
		for _, hour := range *hours {
			from := hour.ValidFrom
			p := influxdb2.NewPoint("power",
				map[string]string{"unit": "NOKPerkWh", "currency": "NOK"},
				map[string]interface{}{"last": hour.NOKPerKWh * 1.25},
				from,
			)
			points = append(points, p)
		}

		bucket := "power"
		writeAPI := client.WriteAPIBlocking(org, bucket)
		err = writeAPI.WritePoint(context.TODO(), points...)
		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func main() {
	nextDay := time.Now().Add(24 * time.Hour).Format("2006-01-02")

	org := "db"
	token := os.Getenv("TOKEN")
	// Store the URL of your InfluxDB instance
	url := "http://influxdb.int.larshaugan.net"
	client := influxdb2.NewClient(url, token)
	queryAPI := client.QueryAPI(org)
	result, err := queryAPI.Query(context.TODO(), fmt.Sprintf(`
	from(bucket: "power")
	  |> range(start: %sT00:00:01Z, stop: %sT23:59:01Z)
	  |> filter(fn: (r) => r["_measurement"] == "power")
	  |> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
	  |> yield(name: "last")	
`, nextDay, nextDay))
	if err != nil {
		fmt.Printf("%+v", err)
		log.Fatal(err)
	}
	if result.Next() {
		log.Printf("data already populated for this date: %s", nextDay)
		for result.Next() {
			if result.TableChanged() {
				fmt.Printf("table %s\n", result.TableMetadata().String())
			}
			fmt.Printf("value: %+v\n", result.Record().Value())
			if result.Err() != nil {
				fmt.Printf("query parsing error: %s\n", result.Err().Error())
			}
		}
	}
	log.Printf("Gathering data for next day: %s", nextDay)
	hours, err := getPower(nextDay)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("data gathered: %+v", *hours)

	var points []*write.Point
	for _, hour := range *hours {
		from := hour.ValidFrom
		p := influxdb2.NewPoint("power",
			map[string]string{"unit": "NOKPerkWh", "currency": "NOK"},
			map[string]interface{}{"last": hour.NOKPerKWh * 1.25},
			from,
		)
		points = append(points, p)
	}

	bucket := "power"
	writeAPI := client.WriteAPIBlocking(org, bucket)
	err = writeAPI.WritePoint(context.TODO(), points...)
	if err != nil {
		log.Fatal(err)
	}
}
