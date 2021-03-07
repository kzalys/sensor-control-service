package types

type SensorStatus struct {
	SensorGroup   string `json:"sensorGroup,omitempty" form:"sensorGroup"`
	SensorAddress string `json:"sensorAddress,omitempty" form:"sensorAddress"`
	PushInterval  int    `json:"pushInterval,omitempty" form:"pushInterval"`
	InfluxHost    string `json:"influxHost,omitempty" form:"influxHost"`
	InfluxPort    string `json:"influxPort,omitempty" form:"influxPort"`
	InfluxOrg     string `json:"influxOrg,omitempty" form:"influxOrg"`
	InfluxBucket  string `json:"influxBucket,omitempty" form:"influxBucket"`
	InfluxToken   string `json:"influxToken,omitempty" form:"influxToken"`
}
