package statistics

type Class struct {
	DisplayName string
	Total       int
}

type YearStatistics struct {
	Total   int
	Months  []int
	Classes []Class
}
