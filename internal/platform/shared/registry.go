package shared

// AllModels collects every platform entity via each entity file's own
// init() — the same self-registration pattern sample-golang-project's
// domain/models/registery.go uses — so the migrations package never has to
// import every module by name just to list its models.
var AllModels []any

func RegisterModel(model any) {
	AllModels = append(AllModels, model)
}
