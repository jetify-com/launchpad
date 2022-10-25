package flex

// Convenience data type that makes it easy to create JSON-like structures in-line.
// Useful when using the k8s unstructured APIs.
//
// Example:
// "spec": flex.Obj{
// 	"containers": flex.List{
// 		{
// 			"name":  "web",
// 			"image": "nginx:1.12",
// 			"ports": flex.List{
// 				{
// 					"name":          "http",
// 					"protocol":      "TCP",
// 					"containerPort": 80,
// 				},
// 			},
// 		},
// 	},
// },

type Obj map[string]any
type List []map[string]any
