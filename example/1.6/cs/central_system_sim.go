package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	ocpp16 "github.com/lorenzodonini/ocpp-go/ocpp1.6"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/firmware"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/localauth"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/reservation"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/ws"
)

// *****************************************************************************


// find the charger id by the charger number and the charger station
func defineDeviceId(chargePointId string, connectorId string) (string) {
	var v string
	ok := false
	if chargePointId == "EVSE_1" {
		v, ok = EVSE1[connectorId]
	} else if chargePointId == "Simulador" {
		v, ok = SimuladorCarregador[connectorId]
	} else if chargePointId == "Simulador2" {
		v, ok = SimuladorCarregador2[connectorId]
	}
	if !ok{
		// v = "DeviceId"
		v = "erro"
	}
	
	return v
}

func defineMQTTTopic(deviceId string)(string){
	
	var messageTopic strings.Builder
	messageTopic.WriteString(path)
	messageTopic.WriteString(deviceId)
	messageTopic.WriteString(`/rx`)

	return messageTopic.String()
}
var (
	// <- Create var for channel
	c             chan string
	c2            chan [2]string
	sbMqttMessage strings.Builder
	// Registration of transactions
	Transaction = map[string]string{}

	// Charger stations
	EVSE1 = map[string]string{
		"0": "EVSE_1",
		"1": "19400577",
		"2": "19743013",
	}
	//Simulator
	SimuladorCarregador = map[string]string{
		"0": "Simulador",
		"1": "Simulador-1",
	}
	//Simulator
	SimuladorCarregador2 = map[string]string{
		"0": "Simulador2",
		"1": "Simulador2-1",
	}

	// path = "OpenDataTelemetry/IMT/EVSE/"
	path = "IMT/EVSE/"
	// OpenDataTelemetry/IMT/EVSE/{DeviceId}/rx


)


var (
	nextTransactionId = 0
)

// TransactionInfo contains info about a transaction
type TransactionInfo struct {
	id          int
	startTime   *types.DateTime
	endTime     *types.DateTime
	startMeter  int
	endMeter    int
	connectorId int
	idTag       string
}

func (ti *TransactionInfo) hasTransactionEnded() bool {
	return ti.endTime != nil && !ti.endTime.IsZero()
}

// ConnectorInfo contains status and ongoing transaction ID for a connector
type ConnectorInfo struct {
	status             core.ChargePointStatus
	currentTransaction int
}

func (ci *ConnectorInfo) hasTransactionInProgress() bool {
	return ci.currentTransaction >= 0
}

// ChargePointState contains some simple state for a connected charge point
type ChargePointState struct {
	status            core.ChargePointStatus
	diagnosticsStatus firmware.DiagnosticsStatus
	firmwareStatus    firmware.FirmwareStatus
	connectors        map[int]*ConnectorInfo // No assumptions about the # of connectors
	transactions      map[int]*TransactionInfo
	errorCode         core.ChargePointErrorCode
}

func (cps *ChargePointState) getConnector(id int) *ConnectorInfo {
	ci, ok := cps.connectors[id]
	if !ok {
		ci = &ConnectorInfo{currentTransaction: -1}
		cps.connectors[id] = ci
	}
	return ci
}

// CentralSystemHandler contains some simple state that a central system may want to keep.
// In production this will typically be replaced by database/API calls.
type CentralSystemHandler struct {
	chargePoints map[string]*ChargePointState
}

// ------------- Core profile callbacks -------------

func (handler *CentralSystemHandler) OnAuthorize(chargePointId string, request *core.AuthorizeRequest) (confirmation *core.AuthorizeConfirmation, err error) {
	logDefault(chargePointId, request.GetFeatureName()).Infof("client authorized")

	return core.NewAuthorizationConfirmation(types.NewIdTagInfo(types.AuthorizationStatusAccepted)), nil
}

func (handler *CentralSystemHandler) OnBootNotification(chargePointId string, request *core.BootNotificationRequest) (confirmation *core.BootNotificationConfirmation, err error) {
	logDefault(chargePointId, request.GetFeatureName()).Infof("boot confirmed")

	return core.NewBootNotificationConfirmation(types.NewDateTime(time.Now()), defaultHeartbeatInterval, core.RegistrationStatusAccepted), nil
}

func (handler *CentralSystemHandler) OnDataTransfer(chargePointId string, request *core.DataTransferRequest) (confirmation *core.DataTransferConfirmation, err error) {
	logDefault(chargePointId, request.GetFeatureName()).Infof("received data %d", request.Data)

	// type DataTransferRequest struct {
	// 	VendorId  string      `json:"vendorId" validate:"required,max=255"`
	// 	MessageId string      `json:"messageId,omitempty" validate:"max=50"`
	// 	Data      interface{} `json:"data,omitempty"`
	// }

	// m := fmt.Sprintf(
	// 	`{"type":"%s", "VendorId":"%s", "MessageId": "%s", "timestamp": "%s"}`,
	// 	request.GetFeatureName(),
	// 	request.VendorId,
	// 	request.MessageId,
	// 	request.Data,
	// )

	// topic := defineMQTTTopic(deviceId)

	// c2 <- [2]string{topic, m}






	return core.NewDataTransferConfirmation(core.DataTransferStatusAccepted), nil

	///BATERIA
}

func (handler *CentralSystemHandler) OnHeartbeat(chargePointId string, request *core.HeartbeatRequest) (confirmation *core.HeartbeatConfirmation, err error) {
	logDefault(chargePointId, request.GetFeatureName()).Infof("heartbeat handled")

	
	return core.NewHeartbeatConfirmation(types.NewDateTime(time.Now())), nil

}

func (handler *CentralSystemHandler) OnMeterValues(chargePointId string, request *core.MeterValuesRequest) (confirmation *core.MeterValuesConfirmation, err error) {

	logDefault(chargePointId, request.GetFeatureName()).Infof("received meter values for connector %v. Meter values:\n", request.ConnectorId)
	for _, mv := range request.MeterValue {
		logDefault(chargePointId, request.GetFeatureName()).Printf("%v", mv)

	// Message TODO
	// sbMqttMessage.Reset()
	// sbMqttMessage.WriteString(`{"type":"`)
	// sbMqttMessage.WriteString(request.GetFeatureName())
	// sbMqttMessage.WriteString(`", "chargePointId" : "`)
	// sbMqttMessage.WriteString(chargePointId)
	// sbMqttMessage.WriteString(`"}`)

	// m := sbMqttMessage.String()
	// fmt.Printf("\n\n### OnMeterValues: %s", m)

	
	deviceId := defineDeviceId(chargePointId, strconv.Itoa(request.ConnectorId))

	m := fmt.Sprintf(
		`{"type":"%s", "value":"%s", "timestamp": "%s", "unit": "%s", "format": "%s", "measurand":"%s", "context": "%s", "location": "%s", "deviceId": "%s"}`,
		request.GetFeatureName(),
		mv.SampledValue[0].Value,
		mv.Timestamp.String(),
		mv.SampledValue[0].Unit,
		mv.SampledValue[0].Format,
		mv.SampledValue[0].Measurand,
		mv.SampledValue[0].Context,
		mv.SampledValue[0].Location,
		deviceId,
	)

	topic := defineMQTTTopic(deviceId)

	c2 <- [2]string{topic, m}
	myCallback := func(confirmation *core.ChangeAvailabilityConfirmation, e error) {
		if e != nil {
			log.Printf("\n\n\noperation failed: %v", e)
		} else {
			log.Printf("\n\n\n\n\nstatus request MeterValues: %v", confirmation.Status)
			// ... 
		}
	}
	err2 := centralSystem.ChangeAvailability("EVSE_1", myCallback, 1, core.AvailabilityTypeInoperative)
	log.Printf("Sending the second request from meterValues")
	if err2 != nil {
		log.Printf("\n\n\n\n\nerror sending second message: %v", err2)
	}else{
		log.Printf("IT WORKED OUT 2")
	}

}

	return core.NewMeterValuesConfirmation(), nil
}

func (handler *CentralSystemHandler) OnStatusNotification(chargePointId string, request *core.StatusNotificationRequest) (confirmation *core.StatusNotificationConfirmation, err error) {
	info, ok := handler.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charge point %v", chargePointId)
	}
	info.errorCode = request.ErrorCode
	if request.ConnectorId > 0 {
		connectorInfo := info.getConnector(request.ConnectorId)
		connectorInfo.status = request.Status
		logDefault(chargePointId, request.GetFeatureName()).Infof("connector %v updated status to %v", request.ConnectorId, request.Status)
	} else {
		info.status = request.Status
		logDefault(chargePointId, request.GetFeatureName()).Infof("all connectors updated status to %v", request.Status)
	}


	deviceId := defineDeviceId(chargePointId, strconv.Itoa(request.ConnectorId))

	m := fmt.Sprintf(
			`{"type":"%s", "connectorId":"%s", "timestamp": "%s", "status": "%s", "errorCode": "%s", "info":"%s" , "vendorId": "%s","vendorErrorCode":"%s"}`,
			request.GetFeatureName(),
			strconv.Itoa(request.ConnectorId),
			request.Timestamp,
			request.Status,
			request.ErrorCode,
			request.Info,
			request.VendorId,
			request.VendorErrorCode,
		)

	topic := defineMQTTTopic(deviceId)

	c2 <- [2]string{topic, m}


	return core.NewStatusNotificationConfirmation(), nil
}

func (handler *CentralSystemHandler) OnStartTransaction(chargePointId string, request *core.StartTransactionRequest) (confirmation *core.StartTransactionConfirmation, err error) {
	info, ok := handler.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charge point %v", chargePointId)
	}
	connector := info.getConnector(request.ConnectorId)
	if connector.currentTransaction >= 0 {
		return nil, fmt.Errorf("connector %v is currently busy with another transaction", request.ConnectorId)
	}
	transaction := &TransactionInfo{}
	transaction.idTag = request.IdTag               //idTag: O ID da tag do usuário que iniciou a transação.
	transaction.connectorId = request.ConnectorId   //connectorId: O ID do conector onde a transação está ocorrendo.
	transaction.startMeter = request.MeterStart     //startMeter: A leitura inicial do medidor no início da transação.
	transaction.startTime = request.Timestamp       //startTime: O timestamp indicando quando a transação foi iniciada.
	transaction.id = nextTransactionId              //id: Um identificador único para a transação. Este é incrementado usando nextTransactionId.
	nextTransactionId += 1                          //
	connector.currentTransaction = transaction.id   //
	info.transactions[transaction.id] = transaction //

	// type TransactionInfo struct {
	// id          int
	// startTime   *types.DateTime
	// endTime     *types.DateTime			Ñ
	// startMeter  int						ok
	// endMeter    int						Ñ
	// connectorId int						ok
	// idTag       string					ok
	// }
	logDefault(chargePointId, request.GetFeatureName()).Infof("started transaction %v for connector %v", transaction.id, transaction.connectorId)

	deviceId := defineDeviceId(chargePointId, strconv.Itoa(request.ConnectorId))

	m := fmt.Sprintf(
		`{"type":"%s","startMeter":"%s", "transactionId": "%s", "startTime": "%s", "connectorId" : "%s", "IdTag" : "%s"}`,
		request.GetFeatureName(),
		strconv.Itoa(transaction.startMeter),
		strconv.Itoa(transaction.id),
		fmt.Sprint(transaction.startTime),
		strconv.Itoa(request.ConnectorId),
		transaction.idTag,
	)

	topic := defineMQTTTopic(deviceId)

	c2 <- [2]string{topic, m}

	//saving Connectorid from the transaction ID
	Transaction[strconv.Itoa(transaction.id)] = strconv.Itoa(request.ConnectorId)

	// Send mensage to connector
	// request := core.NewDataTransferConfirmation
	// err := centralSystem.SendRequestAsync("clientId", request, callbackFunction)
	// if err != nil {
		// log.Printf("error sending message: %v", err)
	// }

	return core.NewStartTransactionConfirmation(types.NewIdTagInfo(types.AuthorizationStatusAccepted), transaction.id), nil
}

func (handler *CentralSystemHandler) OnStopTransaction(chargePointId string, request *core.StopTransactionRequest) (confirmation *core.StopTransactionConfirmation, err error) {
	info, ok := handler.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charge point %v", chargePointId)
	}
	transaction, ok := info.transactions[request.TransactionId]
	if ok {
		connector := info.getConnector(transaction.connectorId)
		connector.currentTransaction = -1
		transaction.endTime = request.Timestamp
		transaction.endMeter = request.MeterStop
		//TODO: bill charging period to client
	}

	// type TransactionInfo struct {
	// id          int
	// startTime   *types.DateTime
	// endTime     *types.DateTime			Ñ
	// startMeter  int						ok
	// endMeter    int						Ñ
	// connectorId int						ok
	// idTag       string					ok
	// }
	logDefault(chargePointId, request.GetFeatureName()).Infof("stopped transaction %v - %v", request.TransactionId, request.Reason)
	for _, mv := range request.TransactionData {
		logDefault(chargePointId, request.GetFeatureName()).Printf("%v", mv)
	}


	deviceId := defineDeviceId(chargePointId,Transaction[strconv.Itoa(request.TransactionId)])

	m := fmt.Sprintf(
		`{"type":"%s","endMeter":"%s", "transactionId": "%s", "endTime": "%s", "connectorId" : "%s"}`,
		request.GetFeatureName(),
		strconv.Itoa(request.MeterStop),
		strconv.Itoa(request.TransactionId),
		fmt.Sprint(request.Timestamp),
		Transaction[strconv.Itoa(request.TransactionId)],
	)

	topic := defineMQTTTopic(deviceId)

	c2 <- [2]string{topic, m}

	delete(Transaction, strconv.Itoa(request.TransactionId))

	return core.NewStopTransactionConfirmation(), nil
}

// ------------- Firmware management profile callbacks -------------

func (handler *CentralSystemHandler) OnDiagnosticsStatusNotification(chargePointId string, request *firmware.DiagnosticsStatusNotificationRequest) (confirmation *firmware.DiagnosticsStatusNotificationConfirmation, err error) {
	info, ok := handler.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charge point %v", chargePointId)
	}
	info.diagnosticsStatus = request.Status
	logDefault(chargePointId, request.GetFeatureName()).Infof("updated diagnostics status to %v", request.Status)
	return firmware.NewDiagnosticsStatusNotificationConfirmation(), nil
}

func (handler *CentralSystemHandler) OnFirmwareStatusNotification(chargePointId string, request *firmware.FirmwareStatusNotificationRequest) (confirmation *firmware.FirmwareStatusNotificationConfirmation, err error) {
	info, ok := handler.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charge point %v", chargePointId)
	}
	info.firmwareStatus = request.Status
	logDefault(chargePointId, request.GetFeatureName()).Infof("updated firmware status to %v", request.Status)
	return &firmware.FirmwareStatusNotificationConfirmation{}, nil
}

// No callbacks for Local Auth management, Reservation, Remote trigger or Smart Charging profile on central system

// Utility functions

func logDefault(chargePointId string, feature string) *logrus.Entry {
	return log.WithFields(logrus.Fields{"client": chargePointId, "message": feature})
	// return log.WithFields(logrus.Fields{})
}

// *****************************************************************************

const (
	defaultListenPort          = 8887
	defaultHeartbeatInterval   = 600
	envVarServerPort           = "SERVER_LISTEN_PORT"
	envVarTls                  = "TLS_ENABLED"
	envVarCaCertificate        = "CA_CERTIFICATE_PATH"
	envVarServerCertificate    = "SERVER_CERTIFICATE_PATH"
	envVarServerCertificateKey = "SERVER_CERTIFICATE_KEY_PATH"
)

var log *logrus.Logger
var centralSystem ocpp16.CentralSystem

func setupCentralSystem() ocpp16.CentralSystem {
	return ocpp16.NewCentralSystem(nil, nil)
}

func setupTlsCentralSystem() ocpp16.CentralSystem {
	var certPool *x509.CertPool
	// Load CA certificates
	caCertificate, ok := os.LookupEnv(envVarCaCertificate)
	if !ok {
		log.Infof("no %v found, using system CA pool", envVarCaCertificate)
		systemPool, err := x509.SystemCertPool()
		if err != nil {
			log.Fatalf("couldn't get system CA pool: %v", err)
		}
		certPool = systemPool
	} else {
		certPool = x509.NewCertPool()
		data, err := os.ReadFile(caCertificate)
		if err != nil {
			log.Fatalf("couldn't read CA certificate from %v: %v", caCertificate, err)
		}
		ok = certPool.AppendCertsFromPEM(data)
		if !ok {
			log.Fatalf("couldn't read CA certificate from %v", caCertificate)
		}
	}
	certificate, ok := os.LookupEnv(envVarServerCertificate)
	if !ok {
		log.Fatalf("no required %v found", envVarServerCertificate)
	}
	key, ok := os.LookupEnv(envVarServerCertificateKey)
	if !ok {
		log.Fatalf("no required %v found", envVarServerCertificateKey)
	}
	server := ws.NewTLSServer(certificate, key, &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  certPool,
	})
	return ocpp16.NewCentralSystem(nil, server)
}

// Run for every connected Charge Point, to simulate some functionality
func exampleRoutine(chargePointID string, handler *CentralSystemHandler) {
	// Wait for some time
	time.Sleep(2 * time.Second)
	// Reserve a connector
	reservationID := 42
	clientIdTag := "l33t"
	connectorID := 1
	expiryDate := types.NewDateTime(time.Now().Add(1 * time.Hour))
	cb1 := func(confirmation *reservation.ReserveNowConfirmation, err error) {
		if err != nil {
			logDefault(chargePointID, reservation.ReserveNowFeatureName).Errorf("error on request: %v", err)
		} else if confirmation.Status == reservation.ReservationStatusAccepted {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("connector %v reserved for client %v until %v (reservation ID %d)", connectorID, clientIdTag, expiryDate.FormatTimestamp(), reservationID)
		} else {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("couldn't reserve connector %v: %v", connectorID, confirmation.Status)
		}
	}
	e := centralSystem.ReserveNow(chargePointID, cb1, connectorID, expiryDate, clientIdTag, reservationID)
	if e != nil {
		logDefault(chargePointID, reservation.ReserveNowFeatureName).Errorf("couldn't send message: %v", e)
		return
	}
	// Wait for some time
	time.Sleep(1 * time.Second)
	// Cancel the reservation
	cb2 := func(confirmation *reservation.CancelReservationConfirmation, err error) {
		if err != nil {
			logDefault(chargePointID, reservation.CancelReservationFeatureName).Errorf("error on request: %v", err)
		} else if confirmation.Status == reservation.CancelReservationStatusAccepted {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("reservation %v canceled successfully", reservationID)
		} else {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("couldn't cancel reservation %v", reservationID)
		}
	}
	e = centralSystem.CancelReservation(chargePointID, cb2, reservationID)
	if e != nil {
		logDefault(chargePointID, reservation.ReserveNowFeatureName).Errorf("couldn't send message: %v", e)
		return
	}
	// Wait for some time
	time.Sleep(5 * time.Second)
	// Get current local list version
	cb3 := func(confirmation *localauth.GetLocalListVersionConfirmation, err error) {
		if err != nil {
			logDefault(chargePointID, localauth.GetLocalListVersionFeatureName).Errorf("error on request: %v", err)
		} else {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("current local list version: %v", confirmation.ListVersion)
		}
	}
	e = centralSystem.GetLocalListVersion(chargePointID, cb3)
	if e != nil {
		logDefault(chargePointID, localauth.GetLocalListVersionFeatureName).Errorf("couldn't send message: %v", e)
		return
	}
	// Wait for some time
	time.Sleep(5 * time.Second)
	configKey := "MeterValueSampleInterval"
	configValue := "10"
	// Change meter sampling values time
	cb4 := func(confirmation *core.ChangeConfigurationConfirmation, err error) {
		if err != nil {
			logDefault(chargePointID, core.ChangeConfigurationFeatureName).Errorf("error on request: %v", err)
		} else if confirmation.Status == core.ConfigurationStatusNotSupported {
			logDefault(chargePointID, confirmation.GetFeatureName()).Warnf("couldn't update configuration for unsupported key: %v", configKey)
		} else if confirmation.Status == core.ConfigurationStatusRejected {
			logDefault(chargePointID, confirmation.GetFeatureName()).Warnf("couldn't update configuration for readonly key: %v", configKey)
		} else {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("updated configuration for key %v to: %v", configKey, configValue)
		}
	}
	e = centralSystem.ChangeConfiguration(chargePointID, cb4, configKey, configValue)
	if e != nil {
		logDefault(chargePointID, localauth.GetLocalListVersionFeatureName).Errorf("couldn't send message: %v", e)
		return
	}

	// Wait for some time
	time.Sleep(5 * time.Second)
	// Trigger a heartbeat message
	cb5 := func(confirmation *remotetrigger.TriggerMessageConfirmation, err error) {
		if err != nil {
			logDefault(chargePointID, remotetrigger.TriggerMessageFeatureName).Errorf("error on request: %v", err)
		} else if confirmation.Status == remotetrigger.TriggerMessageStatusAccepted {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("%v triggered successfully", core.HeartbeatFeatureName)
		} else if confirmation.Status == remotetrigger.TriggerMessageStatusRejected {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("%v trigger was rejected", core.HeartbeatFeatureName)
		}
	}
	e = centralSystem.TriggerMessage(chargePointID, cb5, core.HeartbeatFeatureName)
	if e != nil {
		logDefault(chargePointID, remotetrigger.TriggerMessageFeatureName).Errorf("couldn't send message: %v", e)
		return
	}

	// Wait for some time
	time.Sleep(5 * time.Second)
	// Trigger a diagnostics status notification
	cb6 := func(confirmation *remotetrigger.TriggerMessageConfirmation, err error) {
		if err != nil {
			logDefault(chargePointID, remotetrigger.TriggerMessageFeatureName).Errorf("error on request: %v", err)
		} else if confirmation.Status == remotetrigger.TriggerMessageStatusAccepted {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("%v triggered successfully", firmware.GetDiagnosticsFeatureName)
		} else if confirmation.Status == remotetrigger.TriggerMessageStatusRejected {
			logDefault(chargePointID, confirmation.GetFeatureName()).Infof("%v trigger was rejected", firmware.GetDiagnosticsFeatureName)
		}
	}
	e = centralSystem.TriggerMessage(chargePointID, cb6, firmware.DiagnosticsStatusNotificationFeatureName)
	if e != nil {
		logDefault(chargePointID, remotetrigger.TriggerMessageFeatureName).Errorf("couldn't send message: %v", e)
		return
	}
}

// Start function
func main() {
	// TODO: MAKE A CHANNEL WITH 4 FIELDS: [type, chargePointID, connectorId, data]
	// TODO: USE defineDeviceId(connectorId) TO MOUNT THE TOPIC
	// TODO: RECEIVE EACH VARIABLE AND MOUNT THE JSON MESSAGE
	// <- Make channel and assign to var
	c = make(chan string)
	c2 = make(chan [2]string)

	id := uuid.New().String()
	var sbMqttClientId strings.Builder
	sbMqttClientId.WriteString("ocpp-")
	sbMqttClientId.WriteString(id)

	// pBroker := "mqtt://mqtt.maua.br:1883"
	// pBroker := "mqtt://smartcampus.maua.br:1883"
	pBroker := "mqtt://weblab.maua.br:1883"
	
	pClientId := sbMqttClientId.String()
	pUser := "PUBLIC"
	pPassword := "public"
	pQos := 0

	pOpts := MQTT.NewClientOptions()
	pOpts.AddBroker(pBroker)
	pOpts.SetClientID(pClientId)
	pOpts.SetUsername(pUser)
	pOpts.SetPassword(pPassword)

	pClient := MQTT.NewClient(pOpts)
	if token := pClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", pBroker)
	}

	go func() {
		for {													// send MQTT

			// incoming, ok := <-c2
			incoming := <-c2
			// fmt.Println("Topic: ", incoming[0],"\t Message:",incoming[1])
			incoming[1] =	fmt.Sprintf( `{"props":{"deviceName":"EVSE"},"data":` +incoming[1]+"}")

			// fmt.Println("Topic: ", incoming[0],"\t Message:",`{"props":{"deviceNam
// e":"EVSE","data":` +incoming[1]+"}}")
			fmt.Println(incoming[1])
			token := pClient.Publish(incoming[0], byte(pQos), false,incoming[1])
			token.Wait()

		}
	}()

	// Load config from ENV
	var listenPort = defaultListenPort
	port, _ := os.LookupEnv(envVarServerPort)
	if p, err := strconv.Atoi(port); err == nil {
		listenPort = p
	} else {
		log.Printf("no valid %v environment variable found, using default port", envVarServerPort)
	}
	// Check if TLS enabled
	t, _ := os.LookupEnv(envVarTls)
	tlsEnabled, _ := strconv.ParseBool(t)
	// Prepare OCPP 1.6 central system
	if tlsEnabled {
		centralSystem = setupTlsCentralSystem()
	} else {
		centralSystem = setupCentralSystem()
	}
	// Support callbacks for all OCPP 1.6 profiles
	handler := &CentralSystemHandler{chargePoints: map[string]*ChargePointState{}}
	centralSystem.SetCoreHandler(handler)
	centralSystem.SetLocalAuthListHandler(handler)
	centralSystem.SetFirmwareManagementHandler(handler)
	centralSystem.SetReservationHandler(handler)
	centralSystem.SetRemoteTriggerHandler(handler)
	centralSystem.SetSmartChargingHandler(handler)
	myCallback := func(confirmation *core.ChangeAvailabilityConfirmation, e error) {
		if e != nil {
			log.Printf("\n\n\noperation failed: %v", e)
		} else {
			log.Printf("\n\n\n\n\nstatus: %v", confirmation.Status)
			// ... 
		}
	}

	// Add handlers for dis/connection of charge points
	centralSystem.SetNewChargePointHandler(func(chargePoint ocpp16.ChargePointConnection) {
		handler.chargePoints[chargePoint.ID()] = &ChargePointState{connectors: map[int]*ConnectorInfo{}, transactions: map[int]*TransactionInfo{}}
		log.WithField("client", chargePoint.ID()).Info("new charge point connected")
		// go exampleRoutine(chargePoint.ID(), handler)

		err := centralSystem.ChangeAvailability("EVSE_1", myCallback, 1, core.AvailabilityTypeInoperative)
		log.Printf("Sending the first request")
		if err != nil {
			log.Printf("\n\nerror sending first message: %v", err)
		}else{
			log.Printf("IT WORKED OUT 1")
		}
		err2 := centralSystem.ChangeAvailability("EVSE_1", myCallback, 1, core.AvailabilityTypeInoperative)
		log.Printf("Sending the second request")
		if err2 != nil {
			log.Printf("\n\n\n\n\nerror sending second message: %v", err2)
		}else{
			log.Printf("IT WORKED OUT 2")
		}

	})
	centralSystem.SetChargePointDisconnectedHandler(func(chargePoint ocpp16.ChargePointConnection) {
		log.WithField("client", chargePoint.ID()).Info("charge point disconnected")
		delete(handler.chargePoints, chargePoint.ID())
	})
	ocppj.SetLogger(log.WithField("logger", "ocppj"))
	ws.SetLogger(log.WithField("logger", "websocket"))
	// Run central system
	log.Infof("starting central system on port %v", listenPort)
	centralSystem.Start(listenPort, "/{ws}")
	log.Info("stopped central system")
}

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	// Set this to DebugLevel if you want to retrieve verbose logs from the ocppj and websocket layers
	log.SetLevel(logrus.InfoLevel)
}
