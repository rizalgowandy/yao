package workflow

import (
	"fmt"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/yaoapp/gou"
	gshare "github.com/yaoapp/gou/query/share"
	"github.com/yaoapp/kun/any"
	"github.com/yaoapp/kun/exception"
	"github.com/yaoapp/kun/maps"
	"github.com/yaoapp/xiang/config"
	"github.com/yaoapp/xiang/helper"
	"github.com/yaoapp/xiang/share"
	"github.com/yaoapp/xiang/xlog"
)

// WorkFlows 工作流列表
var WorkFlows = map[string]*WorkFlow{}

// Load 加载数据表格
func Load(cfg config.Config) {
	LoadFrom(cfg.RootWorkFlow, "")
}

// LoadFrom 从特定目录加载
func LoadFrom(dir string, prefix string) {
	if share.DirNotExists(dir) {
		return
	}
	share.Walk(dir, ".json", func(root, filename string) {
		name := prefix + share.SpecName(root, filename)
		content := share.ReadFile(filename)
		_, err := LoadWorkFlow(content, name)
		if err != nil {
			exception.New("%s 工作流格式错误", 400, name).Ctx(filename).Throw()
		}
	})
}

// LoadWorkFlow 载入工作流
func LoadWorkFlow(source []byte, name string) (*WorkFlow, error) {
	workflow := WorkFlow{Name: name, Source: source}
	err := jsoniter.Unmarshal(source, &workflow)
	if err != nil {
		xlog.Println(name)
		xlog.Println(err.Error())
		xlog.Println(string(source))
		return nil, err
	}
	WorkFlows[workflow.Name] = &workflow
	return WorkFlows[workflow.Name], nil
}

// Select 读取已加载图表
func Select(name string) *WorkFlow {
	workflow, has := WorkFlows[name]
	if !has {
		exception.New(
			fmt.Sprintf("工作流:%s; 尚未加载", name),
			400,
		).Throw()
	}
	return workflow
}

// Reload 重新载入工作流
func (workflow *WorkFlow) Reload() *WorkFlow {
	new, err := LoadWorkFlow(workflow.Source, workflow.Name)
	if err != nil {
		exception.New(
			fmt.Sprintf("工作流:%s; 加载失败", workflow.Name),
			400,
		).Throw()
	}
	WorkFlows[workflow.Name] = new
	return new
}

// Process
// 读取工作流 xiang.workflow.Open(uid, name, data_id)
// 读取工作流 xiang.workflow.Find(id)
// 保存工作流 xiang.workflow.Save(uid, name, node, input)
// 进入下一个节点 xiang.workflow.Next(uid, id, input)
// 跳转到指定节点 xiang.workflow.Goto(uid, id, node, input)

// API:
// 读取工作流 GET /api/xiang/workflow/<工作流名称>/open
// 读取工作流 GET /api/xiang/workflow/<工作流名称>/find/:id
// 读取工作流配置 GET /api/xiang/workflow/<工作流名称>/setting
// 调用自定义API POST /api/xiang/workflow/<工作流名称>/<自定义API路由>

// Setting 返回配置信息
func (workflow *WorkFlow) Setting(id int) {}

// SetupAPIs 注册API
func (workflow *WorkFlow) SetupAPIs(id int) {}

// Find 读取给定ID的工作流
// uid 当前处理人ID, id 数据ID
func (workflow *WorkFlow) Find(id int) map[string]interface{} {
	wflow := gou.Select("xiang.workflow")
	res := wflow.MustFind(id, gou.QueryParam{
		Select: []interface{}{
			"data_id", "id", "input", "output", "name",
			"user_id", "users",
			"node_name", "node_status",
			"status",
			"updated_at", "created_at",
		}})
	return res
}

// Open 读取当前工作流(未完成的)
// uid 当前处理人ID, id 数据ID
func (workflow *WorkFlow) Open(uid int, id interface{}) map[string]interface{} {
	wflow := gou.Select("xiang.workflow")
	params := gou.QueryParam{
		Select: []interface{}{
			"data_id", "id", "input", "output", "name",
			"user_id", "users",
			"node_name", "node_status",
			"status",
			"updated_at", "created_at",
		},
		Wheres: []gou.QueryWhere{
			{Column: "name", Value: workflow.Name},
			{Column: "data_id", Value: id},
			{Column: "user_ids", OP: "like", Value: fmt.Sprintf("%%|%d|%%", uid)},
			{Column: "status", Value: "进行中"},
		},
		Limit: 1,
	}
	rows := wflow.MustGet(params)
	if len(rows) > 0 {
		return rows[0]
	}
	return map[string]interface{}{
		"name":        workflow.Name,
		"data_id":     id,
		"node_name":   workflow.Nodes[0].Name,
		"user_id":     uid,
		"users":       []interface{}{uid},
		"status":      "进行中",
		"node_status": "进行中",
		"input":       map[string]interface{}{},
		"output":      map[string]interface{}{},
	}
}

// Save 保存工作流节点数据 此版本使用Like实现
// uid 当前处理人ID, id 数据ID
func (workflow *WorkFlow) Save(uid int, name string, id interface{}, input Input, outputs ...map[string]interface{}) map[string]interface{} {
	wflow := gou.Select("xiang.workflow")
	params := gou.QueryParam{
		Select: []interface{}{"id", "input", "output", "users"},
		Wheres: []gou.QueryWhere{
			{Column: "name", Value: workflow.Name},
			{Column: "data_id", Value: id},
			{Column: "user_ids", OP: "like", Value: fmt.Sprintf("%%|%d|%%", uid)},
			{Column: "status", Value: "进行中"},
		},
		Limit: 1,
	}

	rows := wflow.MustGet(params)
	data := map[string]interface{}{
		"name":      workflow.Name,
		"data_id":   id,
		"node_name": name,
		"user_id":   uid,
	}
	users := []interface{}{uid}
	output := map[string]interface{}{}
	nodeInput := map[string]interface{}{}
	nodeInput[name] = input
	if len(outputs) > 0 {
		output = outputs[0]
	}
	if len(rows) > 0 {
		data["id"] = rows[0].Get("id")
		nodeInput = workflow.MergeData(rows[0].Get("input"), nodeInput)
		users = workflow.MergeUsers(rows[0].Get("users"), users)
		output = workflow.MergeData(rows[0].Get("output"), output)
	} else {
		data["status"] = "进行中"
		data["node_status"] = "进行中"
	}
	data["users"] = users
	data["user_ids"] = workflow.UserIDs(users)
	data["input"] = nodeInput
	data["output"] = output
	id = wflow.MustSave(data)
	return wflow.MustFind(id, gou.QueryParam{})
}

// Status 标记工作流状态
// uid 当前处理人ID, id 工作流ID
func (workflow *WorkFlow) Status(uid int, id int, output map[string]interface{}) {
}

// Next 下一个工作流
// uid 当前处理人ID, id 工作流ID
func (workflow *WorkFlow) Next(uid int, id int, output map[string]interface{}) map[string]interface{} {
	wflow := workflow.Find(id)
	currNode, ok := wflow["node_name"].(string)
	if !ok {
		exception.New("流程数据异常: 当前节点信息错误", 500).Ctx(currNode).Throw()
	}

	output = workflow.MergeData(wflow["output"], output)
	users := workflow.MergeUsers(wflow["users"], []interface{}{uid})
	// 读取下一个节点
	data := map[string]interface{}{
		"$in":     wflow["input"],
		"$input":  wflow["input"],
		"$out":    output,
		"$outupt": output,
		"$data":   output,
	}
	nextNode := workflow.nextNode(currNode, data)
	nextUID := nextNode.GetUID()

	// 更新数据
	mod := gou.Select("xiang.workflow")
	mod.Save(map[string]interface{}{
		"id":          wflow["id"],
		"output":      wflow["output"],
		"node_name":   nextNode.Name,
		"node_status": "进行中",
		"user_id":     nextUID,
		"users":       users,
		"user_ids":    workflow.UserIDs(users),
	})
	return workflow.Find(id)
}

// GetUID 根据条件选择节点处理人
func (node *Node) GetUID() int {
	res := gou.NewProcess(node.User.Process, node.User.Args...).Run()
	return any.Of(res).CInt()
}

// nextNode 查找下一个节点
func (workflow *WorkFlow) nextNode(currentNode string, data map[string]interface{}) *Node {
	var curr *Node
	nextIndex := -1
	for i, node := range workflow.Nodes {
		if node.Name == currentNode {
			nextIndex = i + 1
			curr = &node
			break
		}
	}
	if nextIndex < 0 {
		exception.New("流程数据异常: 未找到工作流节点", 500).Ctx(currentNode).Throw()
	}

	if nextIndex == workflow.Len() {
		exception.New("流程数据异常: 当前节点为最后一个节点", 500).Ctx(currentNode).Throw()
	}

	// 未声明 Next 节点, 转到下一个节点
	if curr.Next == nil {
		return &workflow.Nodes[nextIndex]
	}

	// 声明 Next 节点, 按条件到指定节点
	data = maps.Of(data).Dot()
	for _, next := range curr.Next {
		node := workflow.GetNodeWhen(next, data)
		if node != nil {
			return node
		}
	}

	exception.New("流程数据异常: 未找到符合条件的工作流节点", 500).Ctx(map[string]interface{}{"current": currentNode, "data": data}).Throw()
	return nil
}

// GetNodeWhen 读取节点
func (workflow *WorkFlow) GetNodeWhen(next Next, data map[string]interface{}) *Node {
	nextNode := ""
	conditions := workflow.Conditions(next.Conditions, data)
	if helper.When(conditions) {
		nextNode = next.Goto
		for i := 0; i < workflow.Len(); i++ {
			node := workflow.Nodes[i]
			if node.Name == nextNode {
				return &node
			}
		}
	}
	return nil
}

// Conditions 处理绑定参数
func (workflow *WorkFlow) Conditions(conds []helper.Condition, data map[string]interface{}) []helper.Condition {
	conditions := []helper.Condition{}
	for _, cond := range conds {
		if left, ok := cond.Left.(string); ok {
			cond.Left = gshare.Bind(left, data)
		}
		if right, ok := cond.Right.(string); ok {
			cond.Right = gshare.Bind(right, data)
		}
		conditions = append(conditions, cond)
	}
	return conditions
}

// UserIDs 读取用户ID
func (workflow *WorkFlow) UserIDs(users []interface{}) string {
	userIDs := []string{}
	for _, u := range users {
		userIDs = append(userIDs, fmt.Sprintf("|%d|", u))
	}
	return strings.Join(userIDs, ",")
}

// MergeUsers 合并数据
func (workflow *WorkFlow) MergeUsers(data interface{}, new interface{}) []interface{} {
	res, ok := data.([]interface{})
	if !ok {
		return []interface{}{}
	}
	if new, ok := new.([]interface{}); ok {
		for _, value := range new {
			data = append(res, value)
		}
	}
	return helper.ArrayUnique(res)
}

// MergeData 合并数据
func (workflow *WorkFlow) MergeData(data interface{}, new interface{}) map[string]interface{} {
	res, ok := data.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}

	if new, ok := new.(map[string]interface{}); ok {
		for key, value := range new {
			res[key] = value
		}
		res = new
	}
	return res
}

// IsLastNode 检查是否为最后一个节点
func (workflow *WorkFlow) IsLastNode(name string) bool {
	length := workflow.Len()
	return workflow.Nodes[length-1].Name == name
}

// Goto 工作流跳转
func (workflow *WorkFlow) Goto(uid int, id int, node string, output map[string]interface{}) {}

// Len 节点数量
func (workflow *WorkFlow) Len() int {
	return len(workflow.Nodes)
}
