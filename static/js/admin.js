var table = new Tabulator("#sensor-list", {
    height: "70%",
    data: adminData != null && adminData.hasOwnProperty("sensors") ? adminData.sensors : [],
    layout: "fitColumns",
    pagination: "local",
    paginationSize: 20,
    paginationSizeSelector: [20, 50, 100, 1000],
    movableColumns: true,
    index: "sensorGroup",
    columns: [
        { title: "Sensor group", field: "sensorGroup", headerFilter: "input", editor: "input" },
        { title: "Sensor address", field: "sensorAddress", headerFilter: "input", editor: "input" },
        { title: "Time since reading", field: "timeSinceReading"},
        { title: "Push interval (ms)", field: "pushInterval", headerFilter: "input", editor: "input" },
        { title: "InfluxDB host", field: "influxHost", headerFilter: "input", editor: "input" },
        { title: "InfluxDB port", field: "influxPort", headerFilter: "input", editor: "input" },
        { title: "InfluxDB org", field: "influxOrg", headerFilter: "input", editor: "input" },
        { title: "InfluxDB bucket", field: "influxBucket", headerFilter: "input", editor: "input" },
        { title: "InfluxDB token", field: "influxToken", headerFilter: "input", editor: "input" },
        { title: "Config fetched at", field: "configFetchTime"},
        { formatter:refreshButton, width:40, align:"center", cellClick:fetchSensorConfig },
        { formatter:"buttonTick", width:40, align:"center", cellClick:updateSensor }
    ]
});

function refreshButton(cell, f, onRendered) {
    var sensorGroup = cell._cell.row.data.sensorGroup;
    onRendered(function () {
        console.log($(".group-" + sensorGroup + " .refresh-btn"));
    })
    return `<i class="fas fa-sync refresh-btn group-${sensorGroup}"></i>`;
}

//create custom formatter
// var cellClassFormatter = function(cell, formatterParams){
//         //cell - the cell component
//         //formatterParams - parameters set for the column
//
//         cell.getElement().addClass("custom-class");
//
//         return cell.getValue(); //return the contents of the cell;
//     },

function fetchSensorConfig(e, cell) {
    var sensor = cell._cell.row.data;

    $.ajax({
        type: "GET",
        url: "/sensors/" + sensor.sensorGroup,
        data: sensor,
        contentType: "application/json",
        success: location.reload,
        error: function (response) {
            alert("Could not fetch config for sensor group " + sensor.sensorGroup + ". Status code " + response.status
                + " received.");
        },
        dataType: "text"
    });
}

function updateSensor(e, cell) {
    var sensor = cell._cell.row.data;

    $.ajax({
        type: "PATCH",
        url: "/sensors/" + sensor.sensorGroup,
        contentType: "application/json",
        data: sensor,
        success: location.reload,
        error: function (response) {
            alert("Could not update config for sensor group " + sensor.sensorGroup + ". Status code " + response.status
                + " received.");
        },
        dataType: "text"
    });
}
