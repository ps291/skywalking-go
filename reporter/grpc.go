package reporter

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent/internal/tool"
	"github.com/dgrijalva/jwt-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	configuration "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	commonv3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	managementv3 "skywalking.apache.org/repo/goapi/collect/management/v3"
)

const (
	maxSendQueueSize     int32 = 30000
	defaultCheckInterval       = 20 * time.Second
	defaultLogPrefix           = "sixthsenseGoAgent-gRPC"
	authKey                    = "Authentication"
	teamidKey                  = "TeamID"
)

// NewGRPCReporter create a new reporter to send data to gRPC oap server. Only one backend address is allowed.
func NewGRPCReporter(serverAddr string, opts ...GRPCReporterOption) (sixthsenseGoAgent.Reporter, error) {
	r := &gRPCReporter{
		logger:        log.New(os.Stderr, defaultLogPrefix, log.LstdFlags),
		sendCh:        make(chan *agentv3.SegmentObject, maxSendQueueSize),
		checkInterval: defaultCheckInterval,
	}
	for _, o := range opts {
		o(r)
	}

	// cds default on
	var credsDialOption grpc.DialOption
	tlsCreds := credentials.NewTLS(&tls.Config{})
	if r.insecure == true {
		credsDialOption = grpc.WithInsecure()
		fmt.Println("Insecure Mode On")
	} else {
		// use tls
		credsDialOption = grpc.WithTransportCredentials(tlsCreds)
	}
	if r.cdsInterval == 0 {
		r.cdsInterval = time.Second * 20
	}

	conn, err := grpc.Dial(serverAddr, credsDialOption)
	if err != nil {
		return nil, err
	}
	r.conn = conn
	r.traceClient = agentv3.NewTraceSegmentReportServiceClient(r.conn)
	r.managementClient = managementv3.NewManagementServiceClient(r.conn)
	if r.cdsInterval > 0 {
		r.cdsClient = configuration.NewConfigurationDiscoveryServiceClient(r.conn)
		r.cdsService = sixthsenseGoAgent.NewConfigDiscoveryService()
	}
	return r, nil
}

// GRPCReporterOption allows for functional options to adjust behaviour
// of a gRPC reporter to be created by NewGRPCReporter
type GRPCReporterOption func(r *gRPCReporter)

// WithLogger setup logger for gRPC reporter
func WithLogger(logger *log.Logger) GRPCReporterOption {
	return func(r *gRPCReporter) {
		r.logger = logger
	}
}

// WithCheckInterval setup service and endpoint registry check interval
func WithCheckInterval(interval time.Duration) GRPCReporterOption {
	return func(r *gRPCReporter) {
		r.checkInterval = interval
	}
}

// WithMaxSendQueueSize setup send span queue buffer length
func WithMaxSendQueueSize(maxSendQueueSize int) GRPCReporterOption {
	return func(r *gRPCReporter) {
		r.sendCh = make(chan *agentv3.SegmentObject, maxSendQueueSize)
	}
}

// WithInstanceProps setup service instance properties eg: org=SkyAPM
func WithInstanceProps(props map[string]string) GRPCReporterOption {
	return func(r *gRPCReporter) {
		r.instanceProps = props
	}
}

// WithTransportCredentials setup transport layer security
// func WithTransportCredentials(insecure bool) GRPCReporterOption {
// 	return func(r *gRPCReporter) {
// 		r.insecure = insecure
// 	}
// }
func WithTransportInsecure(insecure bool) GRPCReporterOption {
	return func(r *gRPCReporter) {
		r.insecure = insecure
	}
}

// WithAuthentication used Authentication for gRPC
func WithAuthentication(auth string) GRPCReporterOption {
	token, _, err := new(jwt.Parser).ParseUnverified(auth, jwt.MapClaims{})
	if err != nil {
		fmt.Println("WithAuthentication: %s", err)
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		fmt.Println(claims["teamId"])
	} else {
		fmt.Println(err)
	}

	return func(r *gRPCReporter) {
		r.service = claims["teamId"].(string)
		r.md = metadata.New(map[string]string{authKey: auth, teamidKey: (claims["teamId"].(string))})

	}
}

// WithCDS setup Configuration Discovery Service to dynamic config
func WithCDS(interval time.Duration) GRPCReporterOption {
	return func(r *gRPCReporter) {
		r.cdsInterval = interval
	}
}

type gRPCReporter struct {
	service          string
	serviceInstance  string
	instanceProps    map[string]string
	logger           *log.Logger
	sendCh           chan *agentv3.SegmentObject
	conn             *grpc.ClientConn
	traceClient      agentv3.TraceSegmentReportServiceClient
	managementClient managementv3.ManagementServiceClient
	checkInterval    time.Duration
	cdsInterval      time.Duration
	cdsService       *sixthsenseGoAgent.ConfigDiscoveryService
	cdsClient        configuration.ConfigurationDiscoveryServiceClient

	md    metadata.MD
	creds credentials.TransportCredentials
	insecure bool
}

type Service struct {
	ServiceName string `json:"name"`
	TeamId      string `json:"teamID"`
	Capability  string `json:"type"`
}

func (r *gRPCReporter) Boot(service string, serviceInstance string, cdsWatchers []sixthsenseGoAgent.AgentConfigChangeWatcher) {
	ser := &Service{
		ServiceName: service,
		TeamId:      r.service,
		Capability:  "A",
	}
	data, _ := json.Marshal(ser)
	serviceName := string(data)
	formattedServiceName := strings.Replace(serviceName, "\"", "'", -1)
	r.service = string(formattedServiceName)
	r.serviceInstance = serviceInstance
	r.initSendPipeline()
	r.check()
	r.initCDS(cdsWatchers)
}

func (r *gRPCReporter) Send(spans []sixthsenseGoAgent.ReportedSpan) {
	spanSize := len(spans)
	if spanSize < 1 {
		return
	}
	rootSpan := spans[spanSize-1]
	rootCtx := rootSpan.Context()
	segmentObject := &agentv3.SegmentObject{
		TraceId:         rootCtx.TraceID,
		TraceSegmentId:  rootCtx.SegmentID,
		Spans:           make([]*agentv3.SpanObject, spanSize),
		Service:         r.service,
		ServiceInstance: r.serviceInstance,
	}
	for i, s := range spans {
		spanCtx := s.Context()
		segmentObject.Spans[i] = &agentv3.SpanObject{
			SpanId:        spanCtx.SpanID,
			ParentSpanId:  spanCtx.ParentSpanID,
			StartTime:     s.StartTime(),
			EndTime:       s.EndTime(),
			OperationName: s.OperationName(),
			Peer:          s.Peer(),
			SpanType:      s.SpanType(),
			SpanLayer:     s.SpanLayer(),
			ComponentId:   s.ComponentID(),
			IsError:       s.IsError(),
			Tags:          s.Tags(),
			Logs:          s.Logs(),
		}
		srr := make([]*agentv3.SegmentReference, 0)
		if i == (spanSize-1) && spanCtx.ParentSpanID > -1 {
			srr = append(srr, &agentv3.SegmentReference{
				RefType:               agentv3.RefType_CrossThread,
				TraceId:               spanCtx.TraceID,
				ParentTraceSegmentId:  spanCtx.ParentSegmentID,
				ParentSpanId:          spanCtx.ParentSpanID,
				ParentService:         r.service,
				ParentServiceInstance: r.serviceInstance,
			})
		}
		if len(s.Refs()) > 0 {
			for _, tc := range s.Refs() {
				srr = append(srr, &agentv3.SegmentReference{
					RefType:                  agentv3.RefType_CrossProcess,
					TraceId:                  spanCtx.TraceID,
					ParentTraceSegmentId:     tc.ParentSegmentID,
					ParentSpanId:             tc.ParentSpanID,
					ParentService:            tc.ParentService,
					ParentServiceInstance:    tc.ParentServiceInstance,
					ParentEndpoint:           tc.ParentEndpoint,
					NetworkAddressUsedAtPeer: tc.AddressUsedAtClient,
				})
			}
		}
		segmentObject.Spans[i].Refs = srr
	}
	defer func() {
		// recover the panic caused by close sendCh
		if err := recover(); err != nil {
			r.logger.Printf("reporter segment err %v", err)
		}
	}()
	select {
	case r.sendCh <- segmentObject:
	default:
		r.logger.Printf("reach max send buffer")
	}
}

func (r *gRPCReporter) Close() {
	if r.sendCh != nil {
		close(r.sendCh)
	}
	r.closeGRPCConn()
}

func (r *gRPCReporter) closeGRPCConn() {
	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			r.logger.Print(err)
		}
	}
}

func (r *gRPCReporter) initSendPipeline() {
	if r.traceClient == nil {
		return
	}
	go func() {
		var retry = 0
	StreamLoop:
		for {
			stream, err := r.traceClient.Collect(metadata.NewOutgoingContext(context.Background(), r.md))
			if err != nil {
				retry++
				if retry > 3 {
					r.closeGRPCConn()
					break
				}
				// fmt.Println("GRPC", err)
				r.logger.Printf("open stream error %v", err)
				time.Sleep(5 * time.Second)
				continue StreamLoop
			}
			for s := range r.sendCh {
				err = stream.Send(s)
				if err != nil {
					r.logger.Printf("send segment error %v", err)
					r.closeStream(stream)
					continue StreamLoop
				}
			}
			r.closeStream(stream)
			r.closeGRPCConn()
			break
		}
	}()
}

func (r *gRPCReporter) initCDS(cdsWatchers []sixthsenseGoAgent.AgentConfigChangeWatcher) {
	if r.cdsClient == nil {
		return
	}

	// bind watchers
	r.cdsService.BindWatchers(cdsWatchers)

	// fetch config
	go func() {
		for {
			if r.conn.GetState() == connectivity.Shutdown {
				break
			}

			configurations, err := r.cdsClient.FetchConfigurations(context.Background(), &configuration.ConfigurationSyncRequest{
				Service: r.service,
				Uuid:    r.cdsService.UUID,
			})

			if err != nil {
				r.logger.Printf("fetch dynamic configuration error %v", err)
				time.Sleep(r.checkInterval)
				continue
			}

			if len(configurations.GetCommands()) > 0 && configurations.GetCommands()[0].Command == "ConfigurationDiscoveryCommand" {
				command := configurations.GetCommands()[0]
				r.cdsService.HandleCommand(command)
			}

			time.Sleep(r.checkInterval)
		}
	}()
}

func (r *gRPCReporter) closeStream(stream agentv3.TraceSegmentReportService_CollectClient) {
	_, err := stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		r.logger.Printf("send closing error %v", err)
	}
}

func (r *gRPCReporter) reportInstanceProperties() (err error) {
	props := buildOSInfo()
	if r.instanceProps != nil {
		for k, v := range r.instanceProps {
			props = append(props, &commonv3.KeyStringValuePair{
				Key:   k,
				Value: v,
			})
		}
	}
	_, err = r.managementClient.ReportInstanceProperties(metadata.NewOutgoingContext(context.Background(), r.md), &managementv3.InstanceProperties{
		Service:         r.service,
		ServiceInstance: r.serviceInstance,
		Properties:      props,
	})
	return err
}

func (r *gRPCReporter) check() {
	if r.checkInterval < 0 || r.conn == nil || r.managementClient == nil {
		return
	}
	go func() {
		instancePropertiesSubmitted := false
		for {
			if r.conn.GetState() == connectivity.Shutdown {
				break
			}

			if !instancePropertiesSubmitted {
				err := r.reportInstanceProperties()
				if err != nil {
					r.logger.Printf("report serviceInstance properties error %v", err)
					time.Sleep(r.checkInterval)
					continue
				}
				instancePropertiesSubmitted = true
			}

			_, err := r.managementClient.KeepAlive(metadata.NewOutgoingContext(context.Background(), r.md), &managementv3.InstancePingPkg{
				Service:         r.service,
				ServiceInstance: r.serviceInstance,
			})

			if err != nil {
				r.logger.Printf("send keep alive signal error %v", err)
			}
			time.Sleep(r.checkInterval)
		}
	}()
}

func buildOSInfo() (props []*commonv3.KeyStringValuePair) {
	processNo := tool.ProcessNo()
	if processNo != "" {
		kv := &commonv3.KeyStringValuePair{
			Key:   "Process No.",
			Value: processNo,
		}
		props = append(props, kv)
	}

	hostname := &commonv3.KeyStringValuePair{
		Key:   "hostname",
		Value: tool.HostName(),
	}
	props = append(props, hostname)

	language := &commonv3.KeyStringValuePair{
		Key:   "language",
		Value: "go",
	}
	props = append(props, language)

	osName := &commonv3.KeyStringValuePair{
		Key:   "OS Name",
		Value: tool.OSName(),
	}
	props = append(props, osName)

	ipv4s := tool.AllIPV4()
	if len(ipv4s) > 0 {
		for _, ipv4 := range ipv4s {
			kv := &commonv3.KeyStringValuePair{
				Key:   "ipv4",
				Value: ipv4,
			}
			props = append(props, kv)
		}
	}
	return
}
