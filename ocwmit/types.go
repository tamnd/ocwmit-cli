package ocwmit

// Course is a single MIT OpenCourseWare course parsed from the OCW sitemap.
type Course struct {
	Rank   int    `json:"rank"`
	Number string `json:"number"` // e.g. "10.01"
	Title  string `json:"title"`  // e.g. "Ethics For Engineers Artificial Intelligence"
	Term   string `json:"term"`   // e.g. "spring-2020"
	Slug   string `json:"slug"`   // full slug from the sitemap
	URL    string `json:"url"`
}
