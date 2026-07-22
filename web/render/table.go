package render

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/1orz/proxy-speedtest/constant"
	"github.com/1orz/proxy-speedtest/download"
	"golang.org/x/image/font"
)

type Theme struct {
	colorgroup [][]int
	bounds     []int
}

var (
	themes = map[string]Theme{
		"original": Theme{
			colorgroup: [][]int{
				{255, 255, 255},
				{128, 255, 0},
				{255, 255, 0},
				{255, 128, 192},
				{255, 0, 0},
			},
			bounds: []int{0, 64 * 1024, 512 * 1024, 4 * 1024 * 1024, 16 * 1024 * 1024},
		},
		"rainbow": Theme{
			colorgroup: [][]int{
				{255, 255, 255},
				{102, 255, 102},
				{255, 255, 102},
				{255, 178, 102},
				{255, 102, 102},
				{226, 140, 255},
				{102, 204, 255},
				{102, 102, 255},
			},
			bounds: []int{0, 64 * 1024, 512 * 1024, 4 * 1024 * 1024, 16 * 1024 * 1024, 24 * 1024 * 1024, 32 * 1024 * 1024, 40 * 1024 * 1024},
		},
	}

	i18n = map[string]string{
		"cn": `{
			"Title":    "Lite SpeedTest 结果表",
			"CreateAt": "测试时间",
			"Traffic":  "总流量: %s. 总时间: %s, 可用节点: [%s]"
		}`,
		"en": `{
			"Title":    "Lite SpeedTest Result Table",
			"CreateAt": "Create At",
			"Traffic":  "Traffic used: %s. Time used: %s, Working Nodes: [%s]"
		}`,
	}
)

type Node struct {
	Id       int    `json:"id"`
	Group    string `json:"group"`
	Remarks  string `json:"remarks"`
	Protocol string `json:"protocol"`
	Ping     string `json:"ping"`
	AvgSpeed int64  `json:"avgSpeed"`
	MaxSpeed int64  `json:"maxSpeed"` // 保留供排序/文本输出用;PNG 表格已不再单列展示
	// 上传只取平均速度进表;MaxUploadSpeed 仍随 JSON 输出但不进 PNG 列。
	UploadSpeed    int64 `json:"uploadSpeed"`
	MaxUploadSpeed int64 `json:"maxUploadSpeed"`
	Success        bool  `json:"success"`
	Traffic        int64 `json:"traffic"`
	Link           string `json:"link,omitempty"` // api only
}

// column 描述 PNG 表格的一列。列集合在 NewTableWithOption 里一次性构建,
// 之后所有布局函数(宽度/竖线/表头/单元格/彩色块)都按同一份切片顺序迭代,
// 消除了旧实现里 CellWidths.toMap() 依赖 map 顺序的隐患。
type column struct {
	key         string
	header      string
	width       float64
	isSpeed     bool                // 有彩色背景块,且值文字强制深色(两种主题都在亮色块上)
	centerValue bool                // 值居中(Group/Remarks 左对齐,其余居中,复刻旧行为)
	value       func(n Node) string
	speedValue  func(n Node) int64 // 仅 isSpeed 列有
}

func trHeader(language, en, cn string) string {
	if language == "cn" {
		return cn
	}
	return en
}

func anyUpload(nodes Nodes) bool {
	for _, n := range nodes {
		if n.UploadSpeed > 0 {
			return true
		}
	}
	return false
}

// buildColumns 依据语言与是否含上传测速,产出列定义并计算各列宽度。
func buildColumns(face font.Face, nodes Nodes, language string, hasUpload bool) []column {
	cols := []column{
		{key: "Group", header: trHeader(language, "Group", "群组名"),
			value: func(n Node) string { return n.Group }},
		{key: "Remarks", header: trHeader(language, "Remarks", "备注"),
			value: func(n Node) string { return n.Remarks }},
		{key: "Protocol", header: trHeader(language, "Protocol", "协议"), centerValue: true,
			value: func(n Node) string { return n.Protocol }},
		{key: "Ping", header: trHeader(language, "Ping", "Ping"), centerValue: true,
			value: func(n Node) string { return n.Ping }},
		{key: "Speed", header: trHeader(language, "Speed", "速度"), isSpeed: true, centerValue: true,
			value:      func(n Node) string { return download.ByteCountIECTrim(n.AvgSpeed) },
			speedValue: func(n Node) int64 { return n.AvgSpeed }},
	}
	if hasUpload {
		cols = append(cols, column{
			key: "Upload", header: trHeader(language, "Upload", "上传"), isSpeed: true, centerValue: true,
			value:      func(n Node) string { return download.ByteCountIECTrim(n.UploadSpeed) },
			speedValue: func(n Node) int64 { return n.UploadSpeed },
		})
	}
	for i := range cols {
		cols[i].width = colWidth(face, cols[i], nodes)
	}
	return cols
}

// colWidth 取本列表头与全部节点值的最大字符宽度。
func colWidth(face font.Face, c column, nodes Nodes) float64 {
	w := getWidth(face, c.header)
	for _, n := range nodes {
		if cw := getWidth(face, c.value(n)); cw > w {
			w = cw
		}
	}
	return w
}

type Nodes []Node

func (nodes Nodes) Sort(sortMethod string) {
	sort.Slice(nodes[:], func(i, j int) bool {
		switch sortMethod {
		case "speed":
			return nodes[i].MaxSpeed < nodes[j].MaxSpeed
		case "rspeed":
			return nodes[i].MaxSpeed > nodes[j].MaxSpeed
		case "ping":
			return nodes[i].Ping < nodes[j].Ping
		case "rping":
			return nodes[i].Ping > nodes[j].Ping
		default:
			return true
		}
	})
}

func CSV2Nodes(path string) (Nodes, error) {
	recordFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer recordFile.Close()
	reader := csv.NewReader(recordFile)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	nodes := make(Nodes, len(records))
	for i, v := range records {
		if len(v) < 6 {
			continue
		}
		avg, err := strconv.Atoi(v[4])
		if err != nil {
			continue
		}
		max, err := strconv.Atoi(v[5])
		if err != nil {
			continue
		}
		nodes[i] = Node{
			Group:    v[0],
			Remarks:  v[1],
			Protocol: v[2],
			Ping:     v[3],
			AvgSpeed: int64(avg),
			MaxSpeed: int64(max),
		}
	}
	return nodes, nil
}

// IPInfo 是要画进图片页脚的公网出口信息(测速机自身),全局一份。
// 空串表示该族不可用,既不画也不占位。
type IPInfo struct {
	V4Line string
	V6Line string
}

type TableOptions struct {
	horizontalpadding float64 // left + right
	verticalpadding   float64 // up + down
	tableTopPadding   float64 // padding for table
	lineWidth         float64
	fontHeight        float64
	fontSize          int
	smallFontRatio    float64
	fontPath          string
	language          string
	theme             Theme
	timezone          string
	fontBytes         []byte
	appearance        string // "light"(默认,白底黑字) | "dark"(黑底白字)
	ipInfo            IPInfo
}

// SetAppearance 设定深/浅主题。空或未知值按 light 处理。
func (o *TableOptions) SetAppearance(a string) {
	if a == "dark" {
		o.appearance = "dark"
	} else {
		o.appearance = "light"
	}
}

// SetIPInfo 设定图片页脚的公网 IP 展示行(整行文本,已含前缀与 geo)。
func (o *TableOptions) SetIPInfo(v4Line, v6Line string) {
	o.ipInfo = IPInfo{V4Line: v4Line, V6Line: v6Line}
}

func (o TableOptions) bg() (int, int, int) {
	if o.appearance == "dark" {
		return 0x0a, 0x0a, 0x0f
	}
	return 255, 255, 255
}

func (o TableOptions) fg() (int, int, int) {
	if o.appearance == "dark" {
		return 255, 255, 255
	}
	return 0, 0, 0
}

func ipLines(info IPInfo) []string {
	var out []string
	if info.V4Line != "" {
		out = append(out, info.V4Line)
	}
	if info.V6Line != "" {
		out = append(out, info.V6Line)
	}
	return out
}

func NewTableOptions(horizontalpadding float64, verticalpadding float64, tableTopPadding float64,
	lineWidth float64, fontSize int, smallFontRatio float64, fontPath string,
	language string, t string, timezone string, fontBytes []byte) TableOptions {
	theme, ok := themes[t]
	if !ok {
		theme = themes["rainbow"]
	}
	return TableOptions{
		horizontalpadding: horizontalpadding,
		verticalpadding:   verticalpadding,
		tableTopPadding:   tableTopPadding,
		lineWidth:         lineWidth,
		fontSize:          fontSize,
		smallFontRatio:    smallFontRatio,
		fontPath:          fontPath,
		language:          language,
		theme:             theme,
		timezone:          timezone,
		fontBytes:         fontBytes,
		appearance:        "light",
	}
}

type I18N struct {
	CreateAt string
	Title    string
	Traffic  string
}

func NewI18N(data string) (*I18N, error) {
	i18n := &I18N{}
	err := json.Unmarshal([]byte(data), i18n)
	if err != nil {
		return nil, err
	}
	return i18n, nil
}

type Table struct {
	width  int
	height int
	*Context
	nodes   Nodes
	options TableOptions
	columns []column
	i18n    *I18N
}

func NewTable(width int, height int, options TableOptions) Table {
	dc := NewContext(width, height)
	return Table{
		width:   width,
		height:  height,
		Context: dc,
		options: options,
	}
}

func DefaultTable(nodes Nodes, fontPath string) (*Table, error) {
	options := NewTableOptions(40, 30, 0.5, 0.5, 24, 0.5, fontPath, "en", "rainbow", "Asia/Shanghai", nil)
	return NewTableWithOption(nodes, &options)
}

// TODO: load font by name
func NewTableWithOption(nodes Nodes, options *TableOptions) (*Table, error) {
	fontSize := options.fontSize
	fontPath := options.fontPath
	fontface, err := LoadFontFaceByBytes(options.fontBytes, fontPath, float64(fontSize))
	if err != nil {
		return nil, err
	}
	cols := buildColumns(fontface, nodes, options.language, anyUpload(nodes))
	fontHeight := calcHeight(fontface)
	options.fontHeight = fontHeight
	horizontalpadding := options.horizontalpadding

	tableWidth := options.lineWidth * 2
	for _, c := range cols {
		tableWidth += c.width + horizontalpadding
	}
	// IP 行可能比表格更宽,按需加宽(留出左右边距)。
	for _, line := range ipLines(options.ipInfo) {
		if w := getWidth(fontface, line) + horizontalpadding; w > tableWidth {
			tableWidth = w
		}
	}

	n := len(ipLines(options.ipInfo))
	// +4 = 标题/表头/流量/生成时间四行;+n = IP 行(画在表格框下方,poweredBy 之上)。
	tableHeight := (fontHeight+options.verticalpadding)*float64(len(nodes)+4+n) + options.tableTopPadding*2 + options.fontHeight*options.smallFontRatio
	table := NewTable(int(tableWidth), int(tableHeight), *options)
	table.nodes = nodes
	table.columns = cols
	result, err := NewI18N(i18n[options.language])
	if err != nil {
		return nil, err
	}
	table.i18n = result
	table.SetFontFace(fontface)
	return &table, nil
}

func (t *Table) drawHorizonLines() {
	y := t.options.tableTopPadding
	for i := 0; i <= len(t.nodes)+4; i++ {
		t.drawHorizonLine(y)
		y += t.options.fontHeight + t.options.verticalpadding
	}
}

func (t *Table) drawHorizonLine(y float64) {
	t.DrawLine(0, y, float64(t.width), y)
	t.SetLineWidth(t.options.lineWidth)
	t.Stroke()
}

func (t *Table) drawVerticalLines() {
	padding := t.options.horizontalpadding
	var x float64
	t.drawFullVerticalLine(t.options.lineWidth)
	for i := 0; i < len(t.columns)-1; i++ {
		x += t.columns[i].width + padding
		t.drawVerticalLine(x)
	}
	x += t.columns[len(t.columns)-1].width + padding
	t.drawFullVerticalLine(x)
}

func (t *Table) drawVerticalLine(x float64) {
	height := (t.options.fontHeight+t.options.verticalpadding)*float64((len(t.nodes)+2)) + t.options.tableTopPadding
	y := t.options.tableTopPadding + t.options.fontHeight + t.options.verticalpadding
	t.DrawLine(x, y, x, height)
	t.SetLineWidth(t.options.lineWidth)
	t.Stroke()
}

func (t *Table) drawFullVerticalLine(x float64) {
	height := (t.options.fontHeight+t.options.verticalpadding)*float64((len(t.nodes)+4)) + t.options.tableTopPadding
	y := t.options.tableTopPadding
	t.DrawLine(x, y, x, height)
	t.SetLineWidth(t.options.lineWidth)
	t.Stroke()
}

func (t *Table) drawTitle() {
	title := t.i18n.Title
	var x float64 = float64(t.width)/2 - getWidth(t.fontFace, title)/2
	var y float64 = t.options.fontHeight/2 + t.options.verticalpadding/2 + t.options.tableTopPadding
	t.centerString(title, x, y)
}

func (t *Table) drawHeader() {
	horizontalpadding := t.options.horizontalpadding
	var x float64 = horizontalpadding / 2
	var y float64 = t.options.fontHeight/2 + t.options.verticalpadding/2 + t.options.tableTopPadding + t.options.fontHeight + t.options.verticalpadding
	for _, c := range t.columns {
		adjust := c.width/2 - getWidth(t.fontFace, c.header)/2
		t.centerString(c.header, x+adjust, y)
		x += c.width + horizontalpadding
	}
}

func (t *Table) drawTraffic(traffic string) {
	var x float64 = t.options.horizontalpadding / 2
	var y float64 = (t.options.fontHeight+t.options.verticalpadding)*float64((len(t.nodes)+2)) + t.options.tableTopPadding + t.fontHeight/2 + t.options.verticalpadding/2
	t.centerString(traffic, x, y)
}

func (t *Table) FormatTraffic(traffic string, time string, workingNode string) string {
	return fmt.Sprintf(t.i18n.Traffic, traffic, time, workingNode)
}

func (t *Table) drawGeneratedAt() {
	msg := fmt.Sprintf("%s %s", t.i18n.CreateAt, time.Now().Format(time.RFC3339))
	// https://github.com/golang/go/issues/20455
	if runtime.GOOS == "android" {
		loc, _ := time.LoadLocation(t.options.timezone)
		now := time.Now()
		msg = fmt.Sprintf("%s %s", t.i18n.CreateAt, now.In(loc).Format(time.RFC3339))
	}
	var x float64 = t.options.horizontalpadding / 2
	var y float64 = (t.options.fontHeight+t.options.verticalpadding)*float64((len(t.nodes)+3)) + t.options.tableTopPadding + t.fontHeight/2 + t.options.verticalpadding/2
	t.centerString(msg, x, y)
}

// drawIPLines 把公网 IP 行画在表格框下方(len+4+i 行),前景色,主字体。
func (t *Table) drawIPLines() {
	lines := ipLines(t.options.ipInfo)
	if len(lines) == 0 {
		return
	}
	fr, fg, fb := t.options.fg()
	t.SetRGB255(fr, fg, fb)
	x := t.options.horizontalpadding / 2
	for i, line := range lines {
		y := (t.options.fontHeight+t.options.verticalpadding)*float64(len(t.nodes)+4+i) + t.options.tableTopPadding + t.fontHeight/2 + t.options.verticalpadding/2
		t.centerString(line, x, y)
	}
}

func (t *Table) drawPoweredBy() {
	fontSize := int(float64(t.options.fontSize) * t.options.smallFontRatio)
	fontface, err := LoadFontFaceByBytes(t.options.fontBytes, t.options.fontPath, float64(fontSize))
	if err != nil {
		return
	}
	t.SetFontFace(fontface)
	fr, fg, fb := t.options.fg()
	t.SetRGB255(fr, fg, fb)
	msg := constant.Version + " powered by https://github.com/1orz/proxy-speedtest"
	n := len(ipLines(t.options.ipInfo))
	var x float64 = float64(t.width) - getWidth(fontface, msg) - t.options.lineWidth
	var y float64 = (t.options.fontHeight+t.options.verticalpadding)*float64(len(t.nodes)+4+n) + t.options.fontHeight*t.options.smallFontRatio
	t.DrawString(msg, x, y)
}

func (t *Table) centerString(s string, x, y float64) {
	t.DrawStringAnchored(s, x, y, 0, 0.4)
}

func (t *Table) drawNodes() {
	horizontalpadding := t.options.horizontalpadding
	fr, fg, fb := t.options.fg()
	var y float64 = t.options.fontHeight/2 + t.options.verticalpadding/2 + t.options.tableTopPadding + (t.options.fontHeight+t.options.verticalpadding)*2
	for _, v := range t.nodes {
		x := horizontalpadding / 2
		for _, c := range t.columns {
			s := c.value(v)
			if c.isSpeed {
				t.SetRGB255(0, 0, 0) // 亮色块上强制深字,两种主题都可读
			} else {
				t.SetRGB255(fr, fg, fb)
			}
			if c.centerValue {
				adjust := c.width/2 - getWidth(t.fontFace, s)/2
				t.centerString(s, x+adjust, y)
			} else {
				t.centerString(s, x, y)
			}
			x += c.width + horizontalpadding
		}
		y += t.options.fontHeight + t.options.verticalpadding
	}
	t.SetRGB255(fr, fg, fb)
}

func (t *Table) drawSpeed() {
	padding := t.options.horizontalpadding
	var lineWidth float64 = t.options.lineWidth
	var h float64 = t.options.fontHeight + t.options.verticalpadding - 2*lineWidth
	var y0 float64 = t.options.tableTopPadding + lineWidth + (t.options.fontHeight+t.options.verticalpadding)*2
	for ci, c := range t.columns {
		if !c.isSpeed {
			continue
		}
		x := lineWidth
		for j := 0; j < ci; j++ {
			x += t.columns[j].width + padding
		}
		w := c.width + padding - 2*lineWidth
		y := y0
		for i := 0; i < len(t.nodes); i++ {
			t.DrawRectangle(x, y, w, h)
			r, g, b := getSpeedColor(c.speedValue(t.nodes[i]), t.options.theme)
			t.SetRGB255(r, g, b)
			t.Fill()
			y += t.options.fontHeight + t.options.verticalpadding
		}
	}
}

// render 执行完整绘制序列(Draw/Encode 共用,避免两处漂移)。
func (t *Table) render(traffic string) {
	br, bgc, bb := t.options.bg()
	t.SetRGB255(br, bgc, bb)
	t.Clear()
	fr, fg, fb := t.options.fg()
	t.SetRGB255(fr, fg, fb)
	t.drawHorizonLines()
	t.drawVerticalLines()
	t.drawSpeed()          // 彩色块先画(黑字要盖在上面)
	t.SetRGB255(fr, fg, fb) // drawSpeed 会残留最后一次 fill 色,恢复为前景色
	t.drawTitle()
	t.drawHeader()
	t.drawNodes()
	t.drawTraffic(traffic)
	t.drawGeneratedAt()
	t.drawIPLines()
	t.drawPoweredBy()
}

func (t *Table) Draw(path string, traffic string) {
	t.render(traffic)
	t.SavePNG(path)
}

func (t *Table) Encode(traffic string) ([]byte, error) {
	t.render(traffic)
	var buf bytes.Buffer
	err := t.EncodePNG(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t *Table) EncodeB64(traffic string) (string, error) {
	bytes, err := t.Encode(traffic)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes), nil
}

func getSpeedColor(speed int64, theme Theme) (int, int, int) {
	bounds := theme.bounds
	colorgroup := theme.colorgroup
	for i := 0; i < len(bounds)-1; i++ {
		if speed >= int64(bounds[i]) && speed <= int64(bounds[i+1]) {
			level := float64(speed-int64(bounds[i])) / float64(bounds[i+1]-bounds[i])
			return getColor(colorgroup[i], colorgroup[i+1], level)
		}
	}
	l := len(colorgroup)
	return colorgroup[l-1][0], colorgroup[l-1][1], colorgroup[l-1][2]
}

func getColor(lc []int, rc []int, level float64) (int, int, int) {
	r := float64(lc[0])*(1-level) + float64(rc[0])*level
	g := float64(lc[1])*(1-level) + float64(rc[1])*level
	b := float64(lc[2])*(1-level) + float64(rc[2])*level
	return int(r), int(g), int(b)
}

func calcHeight(fontface font.Face) float64 {
	return float64(fontface.Metrics().Height) / 64
}

func getWidth(fontface font.Face, s string) float64 {
	a := font.MeasureString(fontface, s)
	return float64(a >> 6)
}
