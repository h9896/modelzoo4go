package modelzoo

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

// Zkinfo is the information about zookeeper
type Zkinfo struct {
	Path, Onf                                    string
	EndPoint, Topic1, Topic2, TopicMsg, careNode []string
	ReConnect                                    bool
	conn                                         *zk.Conn
	connEvent                                    <-chan zk.Event
}
type nodesMsg struct {
	Topic      map[string]string
	State      map[string]string
	RunningTCP map[string]string
}

var initZk = &Zkinfo{}

// NodeInfo is every node's state
var NodeInfo = &nodesMsg{
	Topic:      make(map[string]string),
	State:      make(map[string]string),
	RunningTCP: make(map[string]string),
}

var seq = 0
var localendpoint string
var hasConn bool

// ZkConn is used to connect zookeeper
func ZkConn(path string, endpoint []string, onf string) (*Zkinfo, error) {
	conn, connEvent, err := zk.Connect(endpoint, time.Second)
	if err != nil {
		return initZk, err
	}
	initZk.Path = path
	initZk.EndPoint = endpoint
	initZk.Onf = onf
	initZk.conn = conn
	initZk.connEvent = connEvent
	return initZk, nil
}

// Zkget is used to get details on zookeeper
func (z Zkinfo) Zkget() (*Zkinfo, error) {
	tmpz, err := z.getCfg()
	if err != nil {
		return &z, err
	}
	tmpz, err = tmpz.getTopic()
	if err != nil {
		return &z, err
	}
	return tmpz, nil
}
func (z Zkinfo) getCfg() (*Zkinfo, error) {
	cfgpath := z.Path + "/cfg/" + z.Onf
	children, _, err := z.conn.Children(cfgpath)
	// Cfg is config on zookeeper
	var cfg = make(map[string][]byte)
	if err != nil {
		return &z, err
	}
	for _, child := range children {
		byt, _, err := z.conn.Get(cfgpath + "/" + child)
		if err != nil {
			return &z, err
		}
		cfg[child] = byt
	}
	if val, ok := cfg["TopicNode1"]; ok {
		z.Topic1 = strings.Split(string(val), "|")
	} else {
		return &z, errors.New("Can't find TopicNode1 in config\r")
	}
	if val, ok := cfg["TopicNode2"]; ok {
		z.Topic2 = strings.Split(string(val), "|")
	} else {
		return &z, errors.New("Can't find TopicNode2 in config\r")
	}
	if val, ok := cfg["TopicMsg"]; ok {
		z.TopicMsg = strings.Split(string(val), "|")
	} else {
		return &z, errors.New("Can't find TopicMsg in config\r")
	}
	if val, ok := cfg["CareNode"]; ok {
		z.careNode = strings.Split(string(val), "|")
	} else {
		return &z, errors.New("Can't find CareNode in config\r")
	}
	if val, ok := cfg["ReConnect"]; ok {
		if strings.ToUpper(string(val)) == "TRUE" {
			z.ReConnect = true
		} else {
			z.ReConnect = false
		}
	} else {
		return &z, errors.New("Can't find ReConnect in config\r")
	}
	initZk = &z
	return &z, nil
}

func (z Zkinfo) getTopic() (*Zkinfo, error) {
	topicpath := z.Path + "/topics"
	topics, _, err := z.conn.Children(topicpath)
	if err != nil {
		return &z, err
	}
	for _, v := range topics {
		singleT := strings.Split(v, ".")
		tp1 := fmt.Sprintf("%s.%s", singleT[0], singleT[1])
		for _, t1Child := range z.Topic1 {
			if t1Child != tp1 {
				continue
			}
			tp2 := fmt.Sprintf("%s.%s", singleT[3], singleT[4])
			for _, t2Child := range z.Topic2 {
				if t2Child != tp2 {
					continue
				}
				bytes, _, err := z.conn.Get(topicpath + "/" + v)
				if err != nil {
					return &z, err
				}
				NodeInfo.Topic[v] = string(bytes)
			}
		}
	}
	return &z, nil
}
func watchAliveChild(onf string) error {
	for {
		select {
		case connE := <-initZk.connEvent:
			{
				fmt.Println("ConEv", connE.State.String())
				// if session broken, try to reconnect.
				if connE.State == zk.StateExpired || connE.State == zk.StateDisconnected {
					fmt.Println("ReConnect ", initZk.ReConnect)
					if initZk.ReConnect {
						// need to stop to watch first.
						fmt.Println("Connstate:", initZk.conn.State().String())
						//z.conn.Close() //This will cause panic: close of closed channel.
						conn, connEvent, err := zk.Connect(initZk.EndPoint, time.Second)
						if err != nil {
							return err
						}
						initZk.conn = conn
						initZk.connEvent = connEvent
						initZk.AliveNodeAddOrUpdate(localendpoint)
					} else {
						initZk.conn.Close()
						os.Exit(0)
						return errors.New("Broken")
					}
				} else {
					_, _, eventAilve, err := initZk.conn.ChildrenW(fmt.Sprintf("%s/AliveNode/%s", initZk.Path, onf))
					if err != nil {
						return err
					}
					fmt.Println("WatchAliveChild on ", onf)
					select {
					case event := <-eventAilve:
						{
							err = initZk.watchProcess(event)
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
}

func (z Zkinfo) watchProcess(event zk.Event) error {
	fmt.Println("*******************")
	fmt.Println("path:", event.Path)
	fmt.Println("type:", event.Type.String())
	fmt.Println("state:", event.State.String())
	fmt.Println("-------------------")
	// compare which one is minimum.
	slince := strings.Split(event.Path, "/")
	onf := slince[len(slince)-1]
	node, _, err := z.conn.Children(event.Path)
	if err != nil {
		return err
	}
	min := 99999
	stringseq := ""
	if len(node) != 0 {
		for _, v := range node {
			t, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			if t < min {
				min = t
				stringseq = v
			}
		}
	}
	// to get the IP address which is running.
	if stringseq != "" {
		bye, _, err := z.conn.Get(fmt.Sprintf("%s/%s", event.Path, stringseq))
		if err != nil {
			return err
		}
		NodeInfo.RunningTCP[onf] = string(bye)
	}
	// to know the status of each node.
	if onf == z.Onf {
		if min == seq {
			NodeInfo.State[z.Onf] = "Run"
		} else {
			NodeInfo.State[z.Onf] = "Standby"
		}
	} else {
		if _, ok := NodeInfo.State[onf]; ok {
			if stringseq != "" {
				NodeInfo.State[onf] = "Run"
			}
		} else {
			NodeInfo.State[onf] = "Die"
		}
	}
	return nil
}
func watchAlive(carenode []string) {
	for _, val := range carenode {
		go func(onf string) {
			err := watchAliveChild(onf)
			if err != nil {
				fmt.Println("WatchAlive on ", err, "onf: ", onf)
				//do something
			}
		}(val)
	}
	go func(onf string) {
		err := watchAliveChild(onf)
		if err != nil {
			fmt.Println("2WatchAlive on ", err, "onf: ", onf)
			//do something
		}
	}(initZk.Onf)
}
func (z Zkinfo) aliveNodeServices() error {
	alivepath := z.Path + "/AliveNode"
	for _, val := range z.careNode {
		node, _, err := z.conn.Children(fmt.Sprintf("%s/%s", alivepath, val))
		if err != nil {
			return err
		}
		if len(node) == 0 {
			NodeInfo.State[val] = "Die"
		} else {
			NodeInfo.State[val] = "Run"
			min := 99999
			stringseq := ""
			if len(node) != 0 {
				for _, v := range node {
					t, err := strconv.Atoi(v)
					if err != nil {
						return err
					}
					if t < min {
						min = t
						stringseq = v
					}
				}
			}
			// to get the IP address which is running.
			if stringseq != "" {
				bye, _, err := z.conn.Get(fmt.Sprintf("%s/%s/%s", alivepath, val, stringseq))
				if err != nil {
					return err
				}
				NodeInfo.RunningTCP[val] = string(bye)
			}
		}
	}
	return nil
}

// AliveNodeAddOrUpdate is used to update the node or add new one at alive path
func (z Zkinfo) AliveNodeAddOrUpdate(endpoint string) error {
	strpath := fmt.Sprintf("%s/AliveNode/%s", z.Path, z.Onf)
	node, _, err := z.conn.Children(strpath)
	if err != nil {
		return err
	}
	min := 99999
	if len(node) != 0 {
		for _, v := range node {
			t, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			if t < min {
				min = t
			}
		}
		if seq == 0 {
			seqpath, err := z.createProtectedSequential(strpath+"/", []byte(endpoint), zk.WorldACL(zk.PermAll))
			if err != nil {
				return err
			}
			tmpslice := strings.Split(seqpath, "/")
			seq, err = strconv.Atoi(tmpslice[len(tmpslice)-1])
			if err != nil {
				return err
			}
			NodeInfo.State[z.Onf] = "Standby"
			localendpoint = endpoint
		} else if seq != min {
			NodeInfo.State[z.Onf] = "Standby"
			localendpoint = endpoint
		} else {
			NodeInfo.State[z.Onf] = "Run"
			localendpoint = endpoint
			NodeInfo.RunningTCP[z.Onf] = localendpoint
		}
	} else {
		seqpath, err := z.createProtectedSequential(strpath+"/", []byte(endpoint), zk.WorldACL(zk.PermAll))
		if err != nil {
			return err
		}
		tmpslice := strings.Split(seqpath, "/")
		seq, err = strconv.Atoi(tmpslice[len(tmpslice)-1])
		if err != nil {
			return err
		}
		NodeInfo.State[z.Onf] = "Run"
		localendpoint = endpoint
		NodeInfo.RunningTCP[z.Onf] = localendpoint
	}
	return nil
}
