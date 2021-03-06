package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/kzalys/sensor-control-service/types"
	"net/http"
	"os"
	"time"
)

const INFLUXDB_HOST = "INFLUXDB_HOST"
const INFLUXDB_ORG = "INFLUXDB_ORG"
const INFLUXDB_BUCKET = "INFLUXDB_BUCKET"
const INFLUXDB_TOKEN = "INFLUXDB_TOKEN"

const DEFAULT_INFLUXDB_HOST = "http://localhost:8086"

const ADMIN_USERNAME = "admin"
const ADMIN_PASSWORD = "admin"

const SENSOR_CONFIG_METRIC_NAME = "sensor_config"

func lookupEnvOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	} else {
		return defaultValue
	}
}

type apiError struct {
	error string `json:"error"`
}

func main() {
	scs := newSensorControlService(lookupEnvOrDefault(INFLUXDB_HOST, DEFAULT_INFLUXDB_HOST), os.Getenv(INFLUXDB_ORG),
		os.Getenv(INFLUXDB_BUCKET), os.Getenv(INFLUXDB_TOKEN))

	r := gin.Default()

	r.Static("static", "static")
	r.LoadHTMLGlob("templates/*/*.gohtml")


	root := r.Group("/", gin.BasicAuth(gin.Accounts{
		ADMIN_USERNAME: ADMIN_PASSWORD,
	}))
	root.GET("/", scs.ServeRoot)
	root.PATCH("/sensors/:sensorGroup", scs.UpdateSensor)
	root.GET("/sensors/:sensorGroup", scs.FetchSensorConfig)

	r.Run(":8000")
	//

	//queryAPI := client.QueryAPI(org)
	//
	//result, err := queryAPI.Query(context.Background(), fmt.Sprintf("from(bucket:\"%s\")|> range(start: -1h) |> filter(fn: (r) => r._measurement == \"stat\")", bucket))
	//if err != nil {
	//	panic(err)
	//}
	//
	//fmt.Println("query successful")
	//
	//for result.Next() {
	//	if result.TableChanged() {
	//		fmt.Printf("table: %s\n", result.TableMetadata().String())
	//	}
	//	fmt.Printf("row: %s\n", result.Record().String())
	//}
}

func newSensorControlService(influxHost, influxOrg, influxBucket, influxToken string) *sensorControlService {
	client := influxdb2.NewClient(influxHost, influxToken)
	return &sensorControlService{
		influxClient:   client,
		influxWriteApi: client.WriteAPI(influxOrg, influxBucket),
	}
}

type sensorControlService struct {
	influxClient influxdb2.Client
	influxWriteApi api.WriteAPI
}

func (*sensorControlService) ServeRoot(ctx *gin.Context) {
	ctx.HTML(200, "test.gohtml", nil)
}

func newSensorConfigPoint(sensor types.SensorStatus) *write.Point {
	return influxdb2.NewPoint(SENSOR_CONFIG_METRIC_NAME, map[string]string{
		"sensor_group": sensor.SensorGroup,
	}, map[string]interface{}{
		"push_interval": sensor.PushInterval,
		"influx_host": sensor.InfluxHost,
		"influx_port": sensor.InfluxPort,
		"influx_org": sensor.InfluxOrg,
		"influx_bucket": sensor.InfluxBucket,
		"sensor_address": sensor.SensorAddress,
	}, time.Now())
}

func (scs *sensorControlService) FetchSensorConfig(ctx *gin.Context) {
	var sensor types.SensorStatus
	err := ctx.BindQuery(&sensor)
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 2)

	ctx.JSON(http.StatusInternalServerError, apiError{error: fmt.Sprintf("Fetching sensor config failed, " +
		"request failed with error %s", err)})
	return

	res, err := http.Get(fmt.Sprintf("http://%s/status", sensor.SensorAddress))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, apiError{error: fmt.Sprintf("Fetching sensor config failed, " +
			"request failed with error %s", err)})
		fmt.Println(err)
		return
	}
	if res.StatusCode / 100 != 2 {
		ctx.JSON(http.StatusInternalServerError, apiError{error: fmt.Sprintf("Fetching sensor config failed, " +
			"sensor returned status code %d", res.StatusCode)})
		fmt.Println(res.Body)
		return
	}

	scs.influxWriteApi.WritePoint(newSensorConfigPoint(sensor))

	ctx.Status(http.StatusOK)
}

func sendPatchRequest(url string, payload interface{}) *http.Response{
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payloadJson))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	return res
}

func (*sensorControlService) UpdateSensor(ctx *gin.Context) {
	var sensor types.SensorStatus
	err := ctx.Bind(&sensor)
	if err != nil {
		panic(err)
	}

	res := sendPatchRequest(fmt.Sprintf("http://%s/status", sensor.SensorAddress), sensor)
	if res.StatusCode / 100 != 2 {
		ctx.JSON(http.StatusInternalServerError, apiError{error: fmt.Sprintf("Updating sensor config failed, " +
			"sensor returned status code %d", res.StatusCode)})
		fmt.Println(res.Body)
	}

	ctx.Status(http.StatusOK)
}