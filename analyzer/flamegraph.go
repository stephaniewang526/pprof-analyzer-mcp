package analyzer

import (
	"fmt"
	"sort" // Keep sort import for potential future use

	"github.com/google/pprof/profile"
)

// nodeKey uniquely identifies a node in the call tree based on function ID.
// Using only function ID aggregates all calls to the same function regardless of call site,
// which is typical for basic flame graphs. More complex keys could include location ID.
type nodeKey struct {
	funcID uint64
}

// tempNode is used during the tree construction process.
type tempNode struct {
	node      *FlameGraphNode
	children  map[nodeKey]*tempNode
	selfValue int64 // Value attributed directly to this node (leaf of a sample stack)
}

// BuildFlameGraphTree converts pprof profile data into a hierarchical FlameGraphNode structure.
// valueIndex specifies which sample value to use (e.g., 0 for samples, 1 for time/bytes).
func BuildFlameGraphTree(p *profile.Profile, valueIndex int) (*FlameGraphNode, error) {
	if valueIndex < 0 || valueIndex >= len(p.SampleType) {
		return nil, fmt.Errorf("invalid value index %d for profile with %d sample types", valueIndex, len(p.SampleType))
	}

	// Use a map to aggregate values for each unique call stack node (function)
	// nodes := make(map[nodeKey]*tempNode) // Not directly used, tree built via root.children
	root := &tempNode{
		// Root node for the flame graph, represents the total profile value.
		// Name "root" is conventional for d3-flame-graph.
		node:      &FlameGraphNode{Name: "root", Value: 0},
		children:  make(map[nodeKey]*tempNode),
		selfValue: 0, // Root itself doesn't have a self value in this model
	}

	totalSampleValue := int64(0)

	for _, sample := range p.Sample {
		value := sample.Value[valueIndex]
		if value == 0 {
			continue // Skip samples with zero value for the selected index
		}
		totalSampleValue += value

		// Process the stack trace in reverse order (caller to callee for flame graph)
		currentNode := root
		for i := len(sample.Location) - 1; i >= 0; i-- {
			loc := sample.Location[i]
			// Aggregate by function for simplicity first.
			// A location can have multiple lines (e.g., due to inlining). We take the first line's function.
			if len(loc.Line) == 0 {
				continue // Skip locations without line info
			}
			line := loc.Line[0]
			fn := line.Function
			if fn == nil {
				// Use a placeholder name if function is unknown
				// Alternatively, could use loc.Address or other identifiers
				fn = &profile.Function{ID: 0, Name: fmt.Sprintf("unknown @ 0x%x", loc.Address)}
				// continue // Or skip lines without function info? Let's use a placeholder.
			}

			key := nodeKey{funcID: fn.ID}
			childNode, exists := currentNode.children[key]
			if !exists {
				childNode = &tempNode{
					node: &FlameGraphNode{
						Name:     fn.Name, // Use function name
						Value:    0,       // Will be calculated later
						Children: []*FlameGraphNode{},
					},
					children:  make(map[nodeKey]*tempNode),
					selfValue: 0,
				}
				currentNode.children[key] = childNode
			}

			// Add the value to the selfValue of the *leaf* node in this sample's stack trace
			// This represents the time/memory spent directly in this function for this sample.
			if i == 0 {
				childNode.selfValue += value
			}

			// Move to the next level in the tree for the next location in the stack
			currentNode = childNode
		}
	}

	// Now, recursively calculate the total value (self + children) for each node
	// and build the final tree structure.
	calculateTotalValueAndBuildTree(root)

	// Set the root's value to the total sample value calculated during the first pass.
	// calculateTotalValueAndBuildTree should also yield the same result if root.selfValue is 0.
	root.node.Value = totalSampleValue

	// Optional: Sort children nodes by value (descending) for potentially better visualization ordering.
	sortChildrenByValue(root.node)

	return root.node, nil
}

// calculateTotalValueAndBuildTree recursively calculates the total value (self + children)
// for each node and constructs the final FlameGraphNode children slice.
func calculateTotalValueAndBuildTree(tn *tempNode) int64 {
	// Start with the value directly attributed to this node
	total := tn.selfValue
	childrenNodes := []*FlameGraphNode{} // Build the final children list here

	for _, childTempNode := range tn.children {
		// Recursively calculate the total value for the child
		childTotal := calculateTotalValueAndBuildTree(childTempNode)
		// Set the final calculated value on the child's FlameGraphNode
		childTempNode.node.Value = childTotal
		// Only include children that ended up with a non-zero total value
		if childTotal > 0 {
			childrenNodes = append(childrenNodes, childTempNode.node)
		}
		// Add the child's total value to the current node's total
		total += childTotal
	}
	// Assign the final list of children to the current node's FlameGraphNode
	tn.node.Children = childrenNodes
	// Return the calculated total value for this node
	return total
}

// sortChildrenByValue recursively sorts the children of a FlameGraphNode by value (descending).
func sortChildrenByValue(node *FlameGraphNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}
	// Sort the immediate children
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Value > node.Children[j].Value // Descending order
	})
	// Recursively sort the children of each child
	for _, child := range node.Children {
		sortChildrenByValue(child)
	}
}
