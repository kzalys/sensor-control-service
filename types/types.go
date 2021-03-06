package types

type SensorStatus struct {
	SensorGroup string `json:"sensorGroup" form:"sensorGroup"`
	SensorAddress string `json:"sensorAddress" form:"sensorAddress"`
	PushInterval string `json:"pushInterval" form:"pushInterval"`
	InfluxHost string `json:"influxHost" form:"influxHost"`
	InfluxPort string `json:"influxPort" form:"influxPort"`
	InfluxOrg string `json:"influxOrg" form:"influxOrg"`
	InfluxBucket string `json:"influxBucket" form:"influxBucket"`
	InfluxToken string `json:"influxToken,omitempty" form:"influxToken"`
}
