package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/widuu/gojson"
	"io"
	"io/ioutil"
	log "github.com/cihub/seelog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Js struct {
	data interface{}
}

const CLIENTERROR = 10

var Sequence uint32

func main() {
	/*
	   if len(os.Args) != 2 {
	           log.Infof("Usage : %s host:port",os.Args[0])
	           os.Exit(1)
	   } */
	//setLogOutput("./log.txt")
	//set client count
	for clientCount := 0; clientCount < 1; clientCount++ {
		go clientProcess()
	}
	for {
		time.Sleep(3600000 * time.Second)
	}
}

func clientProcess() {
	var result uint8
	var conn *net.TCPConn

	err := LoadConfig("/system/etc/conf.json")
	if err != nil {
		fmt.Printf("LoadConfig failed: (%s)", err)
		os.Exit(1)
	}

	logger, err := log.LoggerFromConfigAsFile("/system/etc/log.xml")
	if err != nil {
		fmt.Printf("Load log config failed: (%s)\n", err)
		os.Exit(1)
	}
	log.ReplaceLogger(logger)

	var addSlice []string
	addSlice = strings.Split(Config.Address, ";")
	log.Infof("content: %s\n", addSlice)
	//var err os.Error
	//service := os.Args[1]
	//service := "10.58.65.221:9000"
	for {
		serverIpList := addSlice
		for _, service := range serverIpList {
			log.Infof("try to connect server address :%v\n", service)
			tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
			if err != nil {
				log.Infof("resolve tcp address fail:%v\n", err)
				continue
			}

			conn, err = net.DialTCP("tcp", nil, tcpAddr)
			if err != nil {
				log.Infof("conection fail:%v\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
			break
		}
		if conn != nil {
			defer conn.Close()

			conn.SetNoDelay(true)
			result = messgeProcess(conn)
			if result == uint8(CLIENTERROR) {
				log.Infof("reconnect server")
				continue
			} else {
				break
			}
		} else {
			continue
		}
	}

}

type Packet struct {
	Type uint8
	Ver  uint8
	Seq  uint32
	Len  uint32
	Data []byte
}

type Server struct {
	DeviceId string
}
type Serverslice struct {
	Servers []Server
}

func (this *Packet) GetBytes() (buf []byte) {
	buf = append(buf, Uint8ToBytes(this.Type)...)
	buf = append(buf, Uint8ToBytes(this.Ver)...)
	buf = append(buf, Uint32ToBytes(this.Seq)...)
	buf = append(buf, Uint32ToBytes(this.Len)...)
	buf = append(buf, this.Data...)
	return buf
}

func SendByteStream(conn *net.TCPConn, buf []byte) error {
	log.Infof("send data header:%v", buf[0:10])
	log.Infof("send data body:%s", string(buf[10:]))
	n, err := conn.Write(buf)
	if n != len(buf) || err != nil {
		log.Infof("SendByteStream Write(): %v\n", err)
		return err
	}
	return nil
}


func SendByteStreamKeepAlive(conn *net.TCPConn, buf []byte) error {
        log.Infof("send data header:%v", buf[0:10])
        n, err := conn.Write(buf[0:10])
        if n != len(buf) || err != nil {
                log.Infof("SendByteStream Write(): %v\n", err)
                return err
        }
        return nil
}

type RegisterInfo struct {
	Deviceid string
}

func SendKeepAliveData(conn *net.TCPConn, dataType uint8) error {
	var pac Packet
	pac.Type = dataType
	pac.Seq = uint32(1)
	pac.Ver = 88
	pac.Len = uint32(0)
	var buf []byte = make([]byte, 10)
	buf = append(buf, Uint8ToBytes(pac.Type)...)
	buf = append(buf, Uint8ToBytes(pac.Ver)...)
	buf = append(buf, Uint32ToBytes(pac.Seq)...)
	buf = append(buf, Uint32ToBytes(pac.Len)...)
	log.Infof("client will send keepalive message")
	return SendByteStreamKeepAlive(conn, buf)
}

func SendRegisterData(conn *net.TCPConn, dataType uint8) error {
	var b []byte
	macAddr, err := getMac()
	if err != nil {
		log.Infof("get mac addres failed:%v\n", err)
	}
	b = []byte(macAddr)
	var pac Packet
	pac.Type = dataType
	pac.Seq = Sequence + uint32(1)
	pac.Ver = 88
	pac.Len = uint32(len(b))
	pac.Data = b
	log.Infof("client will send register message")
	return SendByteStream(conn, pac.GetBytes())
}

type ResponseInfo struct {
	Result string
}

func SendResponseData(conn *net.TCPConn, dataType uint8, body []byte, bSeq uint32) error {
	var pac Packet
	pac.Type = dataType
	pac.Seq = bSeq
	pac.Ver = 88
	pac.Len = uint32(len(body))
	pac.Data = body

	log.Infof("client will send response message")
	log.Infof("send response body to server:%s\n", string(body))
	return SendByteStream(conn, pac.GetBytes())
}

func Uint32ToBytes(v uint32) []byte {
	buf := make([]byte, 4)
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
	return buf
}

func BytesToUint32(buf []byte) uint32 {
	v := (uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]))
	return v
}

func Uint8ToBytes(v uint8) []byte {
	buf := make([]byte, 1)
	buf[0] = byte(v)
	return buf
}

func BytesToUint8(buf []byte) uint8 {
	v := (uint8(buf[0]))
	return v
}

// keepalive message
func KeepAlive(conn *net.TCPConn) uint8 {
	ticker := time.NewTicker(50 * time.Second)
	for _ = range ticker.C {
		err := SendKeepAliveData(conn, uint8(0))
		if err != nil {
			log.Infof("send keepalvie data fail:%v\r\n", err)
			return uint8(CLIENTERROR)
		}
		log.Infof(conn.RemoteAddr().String(), "ping.")
	}
	return uint8(0)
}

func messgeProcess(conn *net.TCPConn) uint8 {
	//var register RegisterInfo
	var response ResponseInfo
	//send register message
	err := SendRegisterData(conn, uint8(1))
	if err != nil {
		log.Infof("send register data fail:%v\r\n", err)
		// os.Exit(1)
	}

	var (
		bType  []byte = make([]byte, 1)
		bSeq   []byte = make([]byte, 4)
		bVer   []byte = make([]byte, 1)
		bLen   []byte = make([]byte, 4)
		pacLen uint32
	)
	c := make(chan uint8, 1)
	// send keepalive message every 5 minutes
	go func() {
		var result uint8
		result = KeepAlive(conn)
		if result == uint8(CLIENTERROR) {
			log.Infof("quit message process result is:%d\n", result)
			c <- uint8(CLIENTERROR)
		}
		c <- uint8(0)
	}()

	/*client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*5)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 2))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 2,
		},
	}*/
	// loop for receive message
	for {
		// read message type
		//time.Sleep(5 * time.Second)
		if n, err := io.ReadFull(conn, bType); err != nil && n != 1 {
			log.Infof("Read message type failed: %v\r\n", err)
			select {
			case v := <-c:
				if v == uint8(CLIENTERROR) {
					log.Infof("quit read message process result is:%d\n", v)
					return uint8(CLIENTERROR)
				} else {
					break
				}
			}
			continue
		}
		messageType := BytesToUint8(bType)
		log.Infof("clent receive message type %d", messageType)

		switch messageType {
		// receive register reponse
		case 2:
			if n, err := io.ReadFull(conn, bVer); err != nil && n != 1 {
				log.Infof("Read register message ver failed: %v\r\n", err)
				continue
			}

			if n, err := io.ReadFull(conn, bSeq); err != nil && n != 4 {
				log.Infof("Read register message seq failed: %v\r\n", err)
				continue
			}

			if n, err := io.ReadFull(conn, bLen); err != nil && n != 4 {
				log.Infof("Read register message Len failed: %v\r\n", err)
				continue
			}
			pacLen = BytesToUint32(bLen)
			pacData := make([]byte, pacLen)
			if n, err := io.ReadFull(conn, pacData); err != nil && n != int(pacLen) {
				log.Infof("Read register pacData failed: %v\r\n", err)
				continue
			}

			err = json.Unmarshal([]byte(pacData), &response)
			if err != nil {
				log.Infof("register json decode error: %v\r\n", err)
				continue
			}
		//receive server request
		case 3:
			if n, err := io.ReadFull(conn, bVer); err != nil && n != 1 {
				log.Infof("Read request message ver failed: %v\r\n", err)
				continue
			}

			if n, err := io.ReadFull(conn, bSeq); err != nil && n != 4 {
				log.Infof("Read request message seq failed: %v\r\n", err)
				continue
			}

			if n, err := io.ReadFull(conn, bLen); err != nil && n != 4 {
				log.Infof("Read request message Len failed: %v\r\n", err)
				continue
			}
			pacLen = BytesToUint32(bLen)
			pacData := make([]byte, pacLen)
			if n, err := io.ReadFull(conn, pacData); err != nil && n != int(pacLen) {
				log.Infof("Read request pacData failed: %v\r\n", err)
				continue
			}
			// parse jason
			uidinfo := gojson.Json(string(pacData)).Get("uid").Tostring()
			log.Infof("uid info:%v", uidinfo)
			cmdinfo := gojson.Json(string(pacData)).Get("cmd").Tostring()
			log.Infof("cmd info:%v", cmdinfo)
			request := gojson.Json(cmdinfo).Get("forward").Tostring()
			log.Infof("request info:%v", request)
			var responsebody []byte
			if len(cmdinfo) != 0 {
				log.Infof("enter cmd process branch\n")
				requestbody := bytes.NewBuffer([]byte(request))

				client := &http.Client{
					Transport: &http.Transport{
						Dial: func(netw, addr string) (net.Conn, error) {
							conn, err := net.DialTimeout(netw, addr, time.Second*5)
							if err != nil {
								return nil, err
							}
							conn.SetDeadline(time.Now().Add(time.Second * 2))
							return conn, nil
						},
						ResponseHeaderTimeout: time.Second * 2,
					},
				}

				log.Infof("send http body:%v", cmdinfo)
				if res, err := client.Post("http://127.0.0.1:9999/", "application/json;charset=utf-8", requestbody); err != nil {
					log.Infof("http post request failed - %v\r\n", err)
				} else {
					result, err2 := ioutil.ReadAll(res.Body)
					log.Infof("res body value is :%s\n", res.Body)
					if err2 != nil {
						log.Infof("http response read failed - %v\r\n", err2)
					}
					res.Body.Close()
					log.Infof("post request result:%s\n", result)
					responsebody = result
					cmdHttpResp := CommandHttpResponse{
						Status: 0,
						Result: string(responsebody),
						Descr:  "",
					}

					finalResp, _ := json.Marshal(cmdHttpResp)
					log.Infof("post final response:%s\n", string(finalResp))
					SendResponseData(conn, uint8(4), finalResp, BytesToUint32(bSeq))
				}

			}
			/*
				else{
					fmt.Printf("do not find cmd jason info")
				}*/
		//invalid meessage
		default:
			log.Infof("invalid message type %d", messageType)
		}
	}
	return uint8(0)
}

type CommandHttpResponse struct {
	Status uint8  `json:"status"`
	Result string `json:"result"`
	Descr  string `json:"descr"`
}

func StreamToByte(stream io.Reader) []byte {
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	return buf.Bytes()
}
/*
func setLogOutput(filepath string) {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	logfile, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		log.Infof("%v\r\n", err)
	}
	log.SetOutput(logfile)
}*/

func getMac() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Infof("get mac addres failed:%v\r\n", err)
	}
	var macAddr string
	var mac string
	for _, inter := range interfaces {
		macAddr = inter.HardwareAddr.String()
		if macAddr != "" {
			mac = strings.Replace(macAddr, ":", "", -1)
			log.Infof("mac address:%s", mac)
		}
	}
	return mac, err
}

type ConfigStruct struct {
	Address string `json:"address"`
}

var (
	Config ConfigStruct
)

func LoadConfig(filename string) error {
	r, err := os.Open(filename)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(r)
	err = decoder.Decode(&Config)
	if err != nil {
		return err
	}
	return nil
}
