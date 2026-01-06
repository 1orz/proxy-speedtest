// Package result provides output formatting for test results
package result

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/tester"
	"github.com/1orz/proxy-speedtest/internal/xray"
)

// OutputMode defines the output format
type OutputMode int

const (
	OutputModeJSON  OutputMode = iota // JSON output
	OutputModeText                    // Plain text output
	OutputModeImage                   // Image output (PNG)
	OutputModeNone                    // No output
)

// Node represents a single test result for output
type Node struct {
	ID       int    `json:"id"`
	Group    string `json:"group"`
	Remarks  string `json:"remarks"`
	Protocol string `json:"protocol"`
	Ping     string `json:"ping"`
	AvgSpeed int64  `json:"avgSpeed"`
	MaxSpeed int64  `json:"maxSpeed"`
	Traffic  int64  `json:"traffic,omitempty"`
	Link     string `json:"link,omitempty"`
	Success  bool   `json:"success"`
}

// Nodes is a slice of Node with sorting capabilities
type Nodes []Node

// Output represents the complete test output
type Output struct {
	Nodes        Nodes   `json:"nodes"`
	TotalTraffic int64   `json:"totalTraffic"`
	Duration     string  `json:"duration"`
	SuccessCount int     `json:"successCount"`
	TotalCount   int     `json:"totalCount"`
	GroupName    string  `json:"groupName,omitempty"`
}

// ResultToNode converts a tester.Result to a Node for output
func ResultToNode(result *tester.Result, groupName string) Node {
	node := Node{
		ID:       result.Index,
		Group:    groupName,
		Remarks:  result.Config.GetName(),
		Protocol: getProtocolString(result.Config),
		AvgSpeed: result.AvgSpeed,
		MaxSpeed: result.MaxSpeed,
		Traffic:  result.Traffic,
		Link:     result.Config.Link,
		Success:  result.Success,
	}

	if result.Ping > 0 {
		node.Ping = fmt.Sprintf("%d", result.Ping)
	} else {
		node.Ping = "0"
	}

	return node
}

// ResultsToOutput converts test results to Output structure
func ResultsToOutput(results []*tester.Result, groupName string, duration string) *Output {
	output := &Output{
		Nodes:     make(Nodes, len(results)),
		Duration:  duration,
		GroupName: groupName,
	}

	for i, result := range results {
		if result == nil {
			continue
		}
		output.Nodes[i] = ResultToNode(result, groupName)
		output.TotalTraffic += result.Traffic
		if result.Success {
			output.SuccessCount++
		}
	}
	output.TotalCount = len(results)

	return output
}

// getProtocolString returns a display string for the protocol
func getProtocolString(config *xray.ProxyConfig) string {
	protocol := string(config.Protocol)

	// Add network type for vmess/vless/trojan
	if config.Stream != nil && config.Stream.Network != "" && config.Stream.Network != "tcp" {
		protocol = fmt.Sprintf("%s/%s", protocol, config.Stream.Network)
	}

	return protocol
}

// Sort sorts nodes by the given method
func (nodes Nodes) Sort(method string) {
	switch strings.ToLower(method) {
	case "speed", "avgspeed":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].AvgSpeed > nodes[j].AvgSpeed
		})
	case "rspeed", "ravgspeed":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].AvgSpeed < nodes[j].AvgSpeed
		})
	case "maxspeed":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].MaxSpeed > nodes[j].MaxSpeed
		})
	case "rmaxspeed":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].MaxSpeed < nodes[j].MaxSpeed
		})
	case "ping":
		sort.Slice(nodes, func(i, j int) bool {
			pi, _ := parseInt(nodes[i].Ping)
			pj, _ := parseInt(nodes[j].Ping)
			// Put failed nodes (ping=0) at the end
			if pi == 0 {
				return false
			}
			if pj == 0 {
				return true
			}
			return pi < pj
		})
	case "rping":
		sort.Slice(nodes, func(i, j int) bool {
			pi, _ := parseInt(nodes[i].Ping)
			pj, _ := parseInt(nodes[j].Ping)
			return pi > pj
		})
	case "id":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
	}
}

func parseInt(s string) (int64, error) {
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result, nil
}

// ToJSON converts output to JSON bytes
func (o *Output) ToJSON(indent bool) ([]byte, error) {
	if indent {
		return json.MarshalIndent(o, "", "  ")
	}
	return json.Marshal(o)
}

// WriteJSON writes output to a JSON file
func (o *Output) WriteJSON(filePath string) error {
	data, err := o.ToJSON(true)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// ToText converts output to plain text format
func (o *Output) ToText() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Group: %s\n", o.GroupName))
	sb.WriteString(fmt.Sprintf("Total: %d, Success: %d\n", o.TotalCount, o.SuccessCount))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", o.Duration))
	sb.WriteString(fmt.Sprintf("Traffic: %s\n", tester.FormatBytes(o.TotalTraffic)))
	sb.WriteString("\n")

	for _, node := range o.Nodes {
		status := "✗"
		if node.Success {
			status = "✓"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s | %s | Ping: %sms | Speed: %s\n",
			status,
			node.Remarks,
			node.Protocol,
			node.Ping,
			tester.FormatSpeed(node.AvgSpeed),
		))
	}

	return sb.String()
}

// WriteText writes output to a text file
func (o *Output) WriteText(filePath string) error {
	return os.WriteFile(filePath, []byte(o.ToText()), 0644)
}

// GetWorkingLinks returns links of working nodes
func (o *Output) GetWorkingLinks() []string {
	var links []string
	for _, node := range o.Nodes {
		if node.Success && node.Link != "" {
			links = append(links, node.Link)
		}
	}
	return links
}

