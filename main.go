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
	"net/http"
	"os"
	"strconv"
	"time"
)

const INFLUXDB_HOST = "INFLUXDB_HOST"
const INFLUXDB_ORG = "INFLUXDB_ORG"
const INFLUXDB_BUCKET = "INFLUXDB_BUCKET"
const INFLUXDB_TOKEN = "INFLUXDB_TOKEN"

const DEFAULT_INFLUXDB_HOST = "http://localhost:8086"

const ADMIN_USERNAME_ENV_VAR = "ADMIN_USERNAME"
const ADMIN_PASSWORD_ENV_VAR = "ADMIN_PASSWORD"
const DEFAULT_ADMIN_USERNAME = "admin"
const DEFAULT_ADMIN_PASSWORD = "admin"

func lookupEnvOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	} else {
		return defaultValue
	}
}

type apiError struct {
	Error string `json:"error"`
}

func main() {
	scs := newSensorControlService(lookupEnvOrDefault(INFLUXDB_HOST, DEFAULT_INFLUXDB_HOST), os.Getenv(INFLUXDB_ORG),
		os.Getenv(INFLUXDB_BUCKET), os.Getenv(INFLUXDB_TOKEN))

	r := gin.Default()

	r.Static("static", "static")
	r.LoadHTMLGlob("templates/*/*.gohtml")

	root := r.Group("/", gin.BasicAuth(gin.Accounts{
		lookupEnvOrDefault(ADMIN_USERNAME_ENV_VAR, DEFAULT_ADMIN_USERNAME): lookupEnvOrDefault(ADMIN_PASSWORD_ENV_VAR,
			DEFAULT_ADMIN_PASSWORD),
	}))
	root.GET("/", scs.ServeRoot)
	root.PATCH("/sensors/:sensorGroup", scs.updateSensor)
	root.GET("/configs", scs.serveSensorConfigs)
	root.PATCH("/configs/pushIntervals", scs.scalePushIntervals)

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
	sensors := map[string]*types.SensorStatus{}

	for result.Next() {
		record := result.Record()
		sensorGroup := record.ValueByKey("sensor_group").(string)

		if _, ok := sensors[sensorGroup]; !ok {
			sensors[sensorGroup] = &types.SensorStatus{
				SensorGroup: sensorGroup,
			}
		}

		switch record.Field() {
		case "push_interval":
			sensors[sensorGroup].PushInterval, _ = record.Value().(int64)
		case "influx_host":
			sensors[sensorGroup].InfluxHost = record.Value().(string)
		case "influx_port":
			sensors[sensorGroup].InfluxPort = record.Value().(string)
		case "influx_org":
			sensors[sensorGroup].InfluxOrg = record.Value().(string)
		case "influx_bucket":
			sensors[sensorGroup].InfluxBucket = record.Value().(string)
		case "sensor_address":
			sensors[sensorGroup].SensorAddress = record.Value().(string)
		}
	}

	var sensorsArr []types.SensorStatus
	for _, sensor := range sensors {
		sensorsArr = append(sensorsArr, *sensor)
	}

	return sensorsArr
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
	result, err := scs.influxQueryApi.Query(timedCtx, fmt.Sprintf(`from(bucket: "%s") |> range(start: -365d) |> filter(fn: (r) => r["_measurement"] == "%s") |> yield(name: "last")`, scs.influxBucket, consts.SENSOR_CONFIG_METRIC_NAME))
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
	scale, err := strconv.ParseFloat(ctx.Query("scale"), 64)

	sensors, err := scs.fetchSensorConfigs(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, apiError{Error: fmt.Sprintf("Fetching sensor configs failed, "+
			"request failed with error %s", err)})
		return
	}

	for _, sensor := range sensors {
		sensor.PushInterval = int64(float64(sensor.PushInterval) * scale)
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
