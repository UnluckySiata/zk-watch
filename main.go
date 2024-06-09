package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/go-zookeeper/zk"
)

const (
	NODE = "/a"
)

var (
	servers     = []string{"127.0.0.1:2181", "127.0.0.1:2182", "127.0.0.1:2183"}
	childrenMap = make(map[string][]string)
	lock        = sync.RWMutex{}
)

func main() {
	a := app.New()
	w := a.NewWindow("Zookeeper Watch")
	childrenMap[""] = []string{NODE}

	b := binding.NewString()
	b.Set("Child nodes: 0 direct, 0 all")
	count := widget.NewLabelWithData(b)

	tree := widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			lock.RLock()
			defer lock.RUnlock()
			if v, ok := childrenMap[uid]; ok {
				return v
			}
			return []string{}
		},
		func(uid widget.TreeNodeID) bool {
			lock.RLock()
			defer lock.RUnlock()
			_, ok := childrenMap[uid]
			return ok
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(uid)
		},
	)
	c := container.NewVSplit(count, tree)

	w.SetContent(c)
	w.Resize(fyne.NewSize(400, 400))

	go handle(w, b)
	a.Run()
}

func handle(w fyne.Window, b binding.String) {
	conn, ch, err := zk.Connect(servers, time.Second)
	if err != nil {
		fmt.Printf("Failed to connect to zookeeper: %v\n", err)
		return
	}
	defer conn.Close()

	exists, _, _, err := conn.ExistsW(NODE)
	if err != nil {
		fmt.Printf("Error setting watch: %v\n", err)
		return
	}

	directCount := 0
	allCount := 0
	if exists {
		w.Show()
		lock.Lock()
		c, _, _, err := conn.ChildrenW(NODE)
		if err != nil {
			fmt.Printf("Error setting watch: %v\n", err)
			return
		}

		children := make([]string, 0, 16)
		queue := list.New()
		for _, n := range c {
			directCount++
			path := fmt.Sprintf("%s/%s", NODE, n)
			queue.PushBack(path)
			children = append(children, path)
		}
		childrenMap[NODE] = children

		for e := queue.Front(); e != nil; e = e.Next() {
			allCount++
			v := e.Value.(string)
			c, _, _, err := conn.ChildrenW(v)
			if err != nil {
				fmt.Printf("Error setting watch: %v\n", err)
			}

			children := make([]string, 0, 16)
			for _, n := range c {
				path := fmt.Sprintf("%s/%s", v, n)
				queue.PushBack(path)
				children = append(children, path)
			}
			childrenMap[v] = children
		}
		lock.Unlock()
		text := fmt.Sprintf("Child nodes: %d direct, %d all", directCount, allCount)
		b.Set(text)
	}

	for e := range ch {
		fmt.Printf("Event: %v\n", e)

		switch e.Type {
		case zk.EventNodeCreated:
			if e.Path != NODE {
				continue
			}
			conn.ExistsW(e.Path)

			w.Show()

			c, _, _, _ := conn.ChildrenW(e.Path)
			for _, n := range c {
				path := fmt.Sprintf("%s/%s", e.Path, n)
				conn.ChildrenW(path)
			}

		case zk.EventNodeDeleted:
			if e.Path != NODE {
				continue
			}

			w.Hide()
			conn.ExistsW(e.Path)

		case zk.EventNodeChildrenChanged:
			c, _, _, _ := conn.ChildrenW(e.Path)
			directCount = 0
			allCount = 0

			children := make([]string, 0, 16)
			for _, n := range c {
				path := fmt.Sprintf("%s/%s", e.Path, n)
				conn.ChildrenW(path)
				children = append(children, path)
			}
			lock.Lock()
			childrenMap[e.Path] = children
			lock.Unlock()

			lock.RLock()
			for range childrenMap[NODE] {
				directCount++
			}
			for _, values := range childrenMap {
				for range values {
					allCount++
				}
			}
			allCount--
			lock.RUnlock()
			text := fmt.Sprintf("Child nodes: %d direct, %d all", directCount, allCount)
			b.Set(text)
		}
	}
}
