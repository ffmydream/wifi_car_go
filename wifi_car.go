package main

import (
	"fmt"
	"strconv"
	"time"

	"strings"

	"net"
	//"github.com/tarm/goserial"
	"github.com/op/go-logging"
	"github.com/sergiorb/pca9685-golang/device"
	"golang.org/x/exp/io/i2c"
	"log"
	"os/exec"
	//	"time"
)

const (
	I2C_ADDR         = "/dev/i2c-1"
	ADDR_01          = 0x40
	SVO_TYPE         = 180 //舵机类型：最大转角
	SERVO_HORIZ      = 0   //横向舵机
	SERVO_VERTIC     = 1   //纵向舵机
	MOTOR_LEFT_FOR   = 2
	MOTOR_LEFT_BACK  = 3
	MOTOR_RIGHT_FOR  = 4
	MOTOR_RIGHT_BACK = 5
	MIN_PULSE        = 0
	MAX_PULSE        = 4095
)

func setAngle(p *device.Pwm, angle int) {
	offReg := int((0.5 + float32(angle)/SVO_TYPE*2) * 4096 / 20)
	p.SetPulse(0, offReg)
}

func setPercentage(p *device.Pwm, percent int) {
	pulseLength := int((MAX_PULSE-MIN_PULSE)*float32(percent)/100 + MIN_PULSE)

	p.SetPulse(0, pulseLength)
}

func main() {
	ch := make(chan string)
	logger := logging.Logger{}
	dev, err := i2c.Open(&i2c.Devfs{Dev: I2C_ADDR}, ADDR_01)
	if err != nil {
		log.Fatal(err)
	}
	pca := device.NewPCA9685(dev, "Servo Controller", MIN_PULSE, MAX_PULSE, &logger)
	pca.Frequency = 50.0
	pca.Init()

	//simple tcp server
	//1.listen ip+port
	listener, err := net.Listen("tcp", "192.168.1.166:50000")
	if err != nil {
		fmt.Printf("listen fail, err: %v\n", err)
		return
	}
	go pacCtr(pca, ch)
	//2.accept client request
	//3.create goroutine for each request
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("accept fail, err: %v\n", err)
			continue
		}
		//create goroutine for each connect
		go doLink(conn, ch)
	}
}

func doLink(conn net.Conn, ch chan string) {
	defer conn.Close()
	for {
		var buf [128]byte
		n, err := conn.Read(buf[:])
		if err != nil {
			fmt.Printf("read from connect failed, err: %v\n", err)
			break
		}
		str := string(buf[:n])
		ch <- str
	}
}

func pacCtr(pca *device.PCA9685, ch chan string) {
	timeOut := false //pca写0标志位，如果超过时间没有接受到数据，停机，因为只需要发送一次停机指令，配合select的超时选项使用。
	//2个舵机通道
	servo0 := pca.NewPwm(SERVO_HORIZ)
	servo1 := pca.NewPwm(SERVO_VERTIC)
	//4个电机通道
	motorLeftForward := pca.NewPwm(MOTOR_LEFT_FOR)
	motorLeftBackward := pca.NewPwm(MOTOR_LEFT_BACK)
	motorRightForward := pca.NewPwm(MOTOR_RIGHT_FOR)
	motorRightBackward := pca.NewPwm(MOTOR_RIGHT_BACK)
	fmt.Println("Start init servos and motors.")
	setAngle(servo0, 90)
	setAngle(servo1, 90)
	for {
		select {
		case str := <-ch:
			sl := strings.Split(str, ":")
			//			fmt.Printf("type %v,channel %v,value %v.\n", sl[0], sl[1], sl[2])
			switch sl[0] {
			case "srvo":
				data, _ := strconv.Atoi(sl[2])
				switch sl[1] {
				case "horiz":
					setAngle(servo0, data)
				case "vertic":
					setAngle(servo1, data)
				}
			case "car":
				data, _ := strconv.Atoi(sl[2])
				switch sl[1] {
				case "for":
					setPercentage(motorLeftBackward, 0)
					setPercentage(motorRightBackward, 0)
					setPercentage(motorLeftForward, data)
					setPercentage(motorRightForward, data)
				case "back":
					setPercentage(motorLeftForward, 0)
					setPercentage(motorRightForward, 0)
					setPercentage(motorLeftBackward, data)
					setPercentage(motorRightBackward, data)
				case "left":
					setPercentage(motorLeftForward, 0)
					setPercentage(motorLeftBackward, data)
					setPercentage(motorRightForward, 0)
					setPercentage(motorRightForward, data)
				case "right":
					setPercentage(motorLeftBackward, 0)
					setPercentage(motorLeftForward, data)
					setPercentage(motorRightForward, 0)
					setPercentage(motorRightBackward, data)
				}
			case "cmd":
				cmd := exec.Command(sl[2])
				err := cmd.Run()
				if err != nil {
					fmt.Println(err.Error())

				}
			}
			timeOut = false
		case <-time.After(400 * time.Millisecond):
			if timeOut == false {
				setPercentage(motorLeftBackward, 0)
				setPercentage(motorLeftForward, 0)
				setPercentage(motorRightForward, 0)
				setPercentage(motorRightBackward, 0)
				//				fmt.Println("timeOut.")
				timeOut = true //操作完成后改变变量状态，以免一直向pca发送停机指令
			}
		}
	}
}
