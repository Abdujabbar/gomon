package gomon

// how to use segments
// seg := gomon.NewSegment("xyz")
// defer seg.Finish()
// for i < 1000 {
// 		ch1 := seg.NewChild("123") ->> 123:file.go:45
//		..... some code here ......
// 		ch1.Finish()
// 		ch2 := seg.NewChild("ch20") ->> ch2:file.go:57
//		...... again some code here ......
// 		ch2.Finish()
//		i++
// }
//
// segment xyz contains:
// -> 123:file.go:45 - total amount of time taken to execute this code segment and avg time
// -> ch2:file.go:57 - total amount of time taken to execute this code segment and avg time
type Segment interface {
	NewChild(name string) Segment
}
