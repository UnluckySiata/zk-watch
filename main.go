package main

import (
	"container/list"
	"fmt"
	"time"

	"github.com/go-zookeeper/zk"
)

const (
	NODE = "/a"
)

var (
	servers     = []string{"127.0.0.1:2181", "127.0.0.1:2182", "127.0.0.1:2183"}
	childrenMap = make(map[string][]string)
)

func main() {
	conn, ch, err := zk.Connect(servers, time.Second)
	if err != nil {
		fmt.Printf("Failed to connect to zookeeper: %v\n", err)
		return
	}
	defer conn.Close()

	open := false
	exists, _, _, err := conn.ExistsW(NODE)
	if err != nil {
		fmt.Printf("Error setting watch: %v\n", err)
		return
	}

	if exists {
		open = true
		c, _, _, err := conn.ChildrenW(NODE)
		if err != nil {
			fmt.Printf("Error setting watch: %v\n", err)
			return
		}

		children := make([]string, 16)
		queue := list.New()
		for _, n := range c {
			path := fmt.Sprintf("%s/%s", NODE, n)
			queue.PushBack(path)
			children = append(children, n)
		}
		childrenMap[NODE] = children

		for e := queue.Front(); e != nil; e = e.Next() {
			v := e.Value.(string)
			c, _, _, err := conn.ChildrenW(v)
			if err != nil {
				fmt.Printf("Error setting watch: %v\n", err)
			}

			children := make([]string, 16)
			for _, n := range c {
				path := fmt.Sprintf("%s/%s", v, n)
				queue.PushBack(path)
				children = append(children, n)
			}
			childrenMap[v] = children
		}
	}

	fmt.Printf("%+v\n", childrenMap)
	for {
		select {
		case e := <-ch:
			fmt.Printf("Event: %v\n", e)

			switch e.Type {
			case zk.EventNodeCreated:
				open = true
				fmt.Printf("Open: %v\n", open)
				conn.ExistsW(e.Path)

				c, _, _, _ := conn.ChildrenW(e.Path)
				for _, n := range c {
					path := fmt.Sprintf("%s/%s", e.Path, n)
					conn.ChildrenW(path)
				}

			case zk.EventNodeDeleted:
				if e.Path == NODE {
					open = false
					fmt.Printf("Open: %v\n", open)
					conn.ExistsW(e.Path)
				}

			case zk.EventNodeChildrenChanged:
				c, _, _, _ := conn.ChildrenW(e.Path)

				children := make([]string, 16)
				for _, n := range c {
					path := fmt.Sprintf("%s/%s", e.Path, n)
					conn.ChildrenW(path)
					children = append(children, n)
				}
				childrenMap[e.Path] = children

			default:
			}

		default:
		}
	}
}
