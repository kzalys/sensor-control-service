package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/kzalys/sensor-control-service/consts"
	"github.com/kzalys/sensor-control-service/types"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const INFLUXDB_HOST_ENV_VAR = "INFLUXDB_HOST"
const INFLUXDB_ORG_ENV_VAR = "INFLUXDB_ORG"
const INFLUXDB_BUCKET_ENV_VAR = "INFLUXDB_BUCKET"
const INFLUXDB_TOKEN_ENV_VAR = "INFLUXDB_TOKEN"
const INFLUXDB_TOKEN_PATH_ENV_VAR = "INFLUXDB_TOKEN_PATH"

const DEFAULT_INFLUXDB_HOST = "http://localhost:8086"

const ADMIN_USERNAME_ENV_VAR = "ADMIN_USERNAME"
const ADMIN_PASSWORD_ENV_VAR = "ADMIN_PASSWORD"
const DEFAULT_ADMIN_USERNAME = "admin"
const DEFAULT_ADMIN_PASSWORD = "admin"

const ACTIVE_SENSOR_CONFIGS_QUERY = `
import "experimental"

sensorConfigs = from(bucket: "iot")
  |> range(start: -365d, stop: now())
  |> filter(fn: (r) => r["_measurement"] == "sensor_config")
  |> pivot(
    rowKey:["_time"],
    columnKey: ["_field"],
    valueColumn: "_value"
  )	
  |> group(columns: ["sensor_group"])
  |> top(n:1, columns: ["_time"])
  |> drop(columns: ["_time", "_start", "_stop", "_measurement"])

latestPushes = from(bucket: "iot")
  |> range(start: -365d, stop: now())
  |> filter(fn: (r) => r["_measurement"] == "sensor_status")
  |> group(columns: ["sensor_group"])
  |> top(n:1, columns: ["_time"])
  |> keep(columns: ["_time", "sensor_group"])
  |> rename(columns: {_time: "last_data_push"})

join(tables: {d1: sensorConfigs, d2: latestPushes}, on: ["sensor_group"], method: "inner")
  |>  map(fn: (r) => ({r with data_lateness: float(v: int(v: experimental.subDuration(d: duration(v: int(v:r.last_data_push)), from: now()))) / float(v: r.push_interval) / float(v: 1000000)}))
  |>  map(fn: (r) => ({r with influx_host: r.influx_host + ":" + r.influx_port}))
  |> filter(fn: (r) => r.data_lateness < 3)
  |> drop(columns: ["data_lateness", "last_data_push"])`

func lookupEnvOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	} else {
		return defaultValue
	}
}
func getInfluxDBToken() string {
	token, ok := os.LookupEnv(INFLUXDB_TOKEN_ENV_VAR)
	if ok {
		return token
	}

	path, ok := os.LookupEnv(INFLUXDB_TOKEN_PATH_ENV_VAR)
	if ok {
		tokenFile, err := os.Open(path)
		if err != nil {
			panic(err)
		}

		token, err := ioutil.ReadAll(tokenFile)
		if err != nil {
			panic(err)
		}

		return strings.TrimSpace(string(token))
	}

	return ""
}

type apiError struct {
	Error string `json:"error"`
}

func main() {
	scs := newSensorControlService(lookupEnvOrDefault(INFLUXDB_HOST_ENV_VAR, DEFAULT_INFLUXDB_HOST), os.Getenv(INFLUXDB_ORG_ENV_VAR),
		os.Getenv(INFLUXDB_BUCKET_ENV_VAR), getInfluxDBToken())

	r := gin.Default()

	r.Static("static", "static")
	r.LoadHTMLGlob("templates/*/*.gohtml")

	root := r.Group("/", gin.BasicAuth(gin.Accounts{
		lookupEnvOrDefault(ADMIN_USERNAME_ENV_VAR, DEFAULT_ADMIN_USERNAME): lookupEnvOrDefault(ADMIN_PASSWORD_ENV_VAR,
			DEFAULT_ADMIN_PASSWORD),
	}))
	root.GET("/", scs.ServeRoot)
	root.PATCH("/configs/:sensorGroup", scs.updateSensor)
	root.GET("/configs", scs.serveSensorConfigs)
	root.PUT("/configs/pushIntervals", scs.scalePushIntervals)

	r.Run(":8000")
}

func newSensorControlService(influxHost, influxOrg, influxBucket, influxToken string) *sensorControlService {
	client := influxdb2.NewClient(influxHost, influxToken)
	return &sensorControlService{
		influxClient:   client,
		influxWriteApi: client.WriteAPI(influxOrg, influxBucket),
		influxQueryApi: client.QueryAPI(influxOrg),
		influxBucket:   influxBucket,
	}
}

type sensorControlService struct {
	influxClient   influxdb2.Client
	influxWriteApi api.WriteAPI
	influxQueryApi api.QueryAPI
	influxBucket   string
}

func (scs *sensorControlService) ServeRoot(ctx *gin.Context) {
	sensors, err := scs.fetchSensorConfigs(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, apiError{Error: fmt.Sprintf("Fetching sensor configs failed, "+
			"request failed with error %s", err)})
		return
	}

	ctx.HTML(200, "test.gohtml", struct {
		Sensors []types.SensorStatus `json:"sensors"`
	}{Sensors: sensors})
}

func newSensorConfigPoint(sensor types.SensorStatus) *write.Point {
	return influxdb2.NewPoint(consts.SENSOR_CONFIG_METRIC_NAME, map[string]string{
		"sensor_group": sensor.SensorGroup,
	}, map[string]interface{}{
		"push_interval":  sensor.PushInterval,
		"influx_host":    sensor.InfluxHost,
		"influx_port":    sensor.InfluxPort,
		"influx_org":     sensor.InfluxOrg,
		"influx_bucket":  sensor.InfluxBucket,
		"sensor_address": sensor.SensorAddress,
	}, time.Now())
}

func queryResultToSensorStatuses(result *api.QueryTableResult) []types.SensorStatus {
	var sensors []types.SensorStatus
	for result.Next() {
		record := result.Record()

		sensors = append(sensors, types.SensorStatus{
			SensorGroup:   record.ValueByKey("sensor_group").(string),
			SensorAddress: record.ValueByKey("sensor_address").(string),
			PushInterval:  record.ValueByKey("push_interval").(int64),
			InfluxHost:    record.ValueByKey("influx_host").(string),
			InfluxPort:    record.ValueByKey("influx_port").(string),
			InfluxOrg:     record.ValueByKey("influx_org").(string),
			InfluxBucket:  record.ValueByKey("influx_bucket").(string),
		})
	}

	return sensors
}

func (scs *sensorControlService) serveSensorConfigs(ctx *gin.Context) {
	sensors, err := scs.fetchSensorConfigs(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, apiError{Error: fmt.Sprintf("Fetching sensor configs failed, "+
			"request failed with error %s", err)})
		return
	}

	ctx.JSON(200, sensors)
}

func (scs *sensorControlService) fetchSensorConfigs(ctx context.Context) ([]types.SensorStatus, error) {
	timedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := scs.influxQueryApi.Query(timedCtx, ACTIVE_SENSOR_CONFIGS_QUERY)
	if err != nil {
		return nil, err
	}

	return queryResultToSensorStatuses(result), nil
}

func sendPatchRequest(url string, payload interface{}) *http.Response {
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

func (scs *sensorControlService) scalePushIntervals(ctx *gin.Context) {
	var res struct {
		Scale float64 `json:"scale"`
	}
	if err := ctx.BindJSON(&res); err != nil {
		ctx.JSON(http.StatusBadRequest, apiError{Error: fmt.Sprintf("could not parse scale: %s", err.Error())})
		return
	}

	if res.Scale == 0 {
		res.Scale, _ = strconv.ParseFloat(ctx.Query("scale"), 64)
	}

	if res.Scale <= 0 {
		ctx.JSON(http.StatusBadRequest, apiError{Error: "scale must be a positive float"})
		return
	}

	sensors, err := scs.fetchSensorConfigs(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, apiError{Error: fmt.Sprintf("Fetching sensor configs failed, "+
			"request failed with error %s", err)})
		return
	}

	for _, sensor := range sensors {
		sensor.PushInterval = int64(float64(sensor.PushInterval) * res.Scale)
		res := sendPatchRequest(fmt.Sprintf("http://%s/status", sensor.SensorAddress), sensor)
		if res.StatusCode/100 != 2 {
			ctx.JSON(http.StatusInternalServerError, apiError{Error: fmt.Sprintf("Updating sensor config failed, "+
				"sensor returned status code %d", res.StatusCode)})
			fmt.Println(res.Body)
		}
	}

	ctx.Status(http.StatusNoContent)
}

func (*sensorControlService) updateSensor(ctx *gin.Context) {
	var sensor types.SensorStatus
	err := ctx.Bind(&sensor)
	if err != nil {
		panic(err)
	}

	res := sendPatchRequest(fmt.Sprintf("http://%s/status", sensor.SensorAddress), sensor)
	if res.StatusCode/100 != 2 {
		ctx.JSON(http.StatusInternalServerError, apiError{Error: fmt.Sprintf("Updating sensor config failed, "+
			"sensor returned status code %d", res.StatusCode)})
		fmt.Println(res.Body)
	}

	ctx.Status(http.StatusOK)
}
