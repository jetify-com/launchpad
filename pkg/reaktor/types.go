package reaktor

// RFC: I was originally going to call this 'Component' (like in React), but ended
// with 'Resource' to more closely match the K8s terminology. Is that the right name?
type Resource interface {
	// TODO:
	//    1. I'd love to return a more specific type. Using any for now so we can
	//       play with typed and untyped APIs, but ideally we choose one.
	ToManifest() (any, error)
}
