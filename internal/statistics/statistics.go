package statistics

type Class struct {
	DisplayName string
	Total       int
}

type Year struct {
	Total   int
	Months  []int
	Classes []Class
}

type Month struct {
	Total   int
	Weeks   []Week
	Classes []Class
}

type Week struct {
	Total int
	// Number is a week number
	Number int
}
