# ai - довідник

Повний довідник пакета `ai`: провайдер-незалежний інтерфейс `Client`, спільна
модель запиту й відповіді, якою «говорить» кожен драйвер, і транспортний
плюмбінг, який перевикористовують автори драйверів.

Англійська версія: **[DOC.md](DOC.md)**.

## Зміст

- [Ментальна модель](#ментальна-модель)
- [Інтерфейс Client](#інтерфейс-client)
- [Побудова запиту](#побудова-запиту)
- [Generate](#generate)
- [Stream](#stream)
- [Інструменти](#інструменти)
- [Мультимодальний ввід](#мультимодальний-ввід)
- [Помилки](#помилки)
- [Для авторів драйверів: Options і транспорт](#для-авторів-драйверів-options-і-транспорт)

## Ментальна модель

Пакет `ai` повторює прийнятий у стандартній бібліотеці поділ на інтерфейс і
драйвери - так само, як `database/sql` відокремлений від своїх драйверів, а
`log/slog` від своїх handler-ів. Цей пакет тримає спільний контракт; окремий
пакет на кожного провайдера (`anthropic`, `openai`, `gemini` тощо) його реалізує.
Драйвер залежить лише від `ai`, тож увесь набір лишається без сторонніх
залежностей.

Код проти інтерфейсу працює з будь-яким провайдером - міняється лише
конструктор, виклики ті самі:

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)

var client ai.Client = anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
```

Ендпоінти, яких провайдери не поділяють (embeddings, генерація зображень, аудіо,
файли, батчі), свідомо відсутні в інтерфейсі. Кожен драйвер подає їх власними
нативними методами, тож спільна поверхня лишається малою й чесною.

## Інтерфейс Client

```go
type Client interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
	Stream(ctx context.Context, req *Request) iter.Seq2[Chunk, error]
}
```

`Generate` повертає цілу `Response`; `Stream` віддає значення `Chunk` у міру
надходження через range-over-func ітератор (Go 1.23+).

## Побудова запиту

`Request` несе модель, необов'язковий system-промпт, розмову й звичні ручки
семплінгу:

```go
type Request struct {
	Model       string
	System      string // необов'язковий system-промпт
	Messages    []Message
	Tools       []Tool
	ToolChoice  ToolChoice
	MaxTokens   int
	Temperature *float64 // вказівник: «не задано» відрізняється від явного 0
	TopP        *float64
	Stop        []string
}
```

Обов'язкові лише `Model` і `Messages`; `Request.Validate` це перевіряє й
повертає `ErrNoModel` чи `ErrNoMessages`. `Temperature` і `TopP` - вказівники,
щоб «не задано» відрізнялося від явного нуля.

`Message` - це роль плюс список частин вмісту `Part`:

```go
type Message struct {
	Role  Role
	Parts []Part
}
```

`Role` - одне з `RoleSystem`, `RoleUser`, `RoleAssistant`, `RoleTool`. Конкретні
частини - `Text`, `Image`, `ToolUse` і `ToolResult`. `Part` - закритий інтерфейс:
його не можна реалізувати поза цим пакетом, тож драйвери роблять вичерпний
switch по ньому.

Конструктори покривають поширений випадок однієї текстової частини:

```go
ai.UserText("Яка столиця Франції?")
ai.SystemText("Ти лаконічний асистент.")
ai.AssistantText("Париж.")
```

## Generate

```go
resp, err := client.Generate(ctx, &ai.Request{
	Model:    "the-model",
	Messages: []ai.Message{ai.UserText("Привітайся.")},
})
if err != nil {
	// обробити помилку
}
fmt.Println(resp.Text())
```

`Response` тримає вихідні блоки асистента плюс службові дані:

```go
type Response struct {
	Model      string
	Parts      []Part
	StopReason string
	Usage      Usage
	Raw        json.RawMessage // оригінальний JSON провайдера
}
```

Хелпери: `Text()` склеює текстові частини, `ToolCalls()` повертає всі виклики
інструментів, а `ToolCall(name)` - перший виклик іменованого інструмента. `Raw`
зберігає оригінальний JSON провайдера для полів, яких цей пакет не моделює.

## Stream

`Stream` повертає `iter.Seq2[Chunk, error]`; ітеруй по ньому:

```go
for chunk, err := range client.Stream(ctx, req) {
	if err != nil {
		// після першої помилки стрім зупиняється
		break
	}
	fmt.Print(chunk.Text)
	if chunk.Done {
		fmt.Printf("\nusage: %+v\n", chunk.Usage)
	}
}
```

```go
type Chunk struct {
	Text     string    // інкрементальна дельта тексту
	ToolCall *ToolUse  // задано, коли чанк несе завершений виклик інструмента
	Usage    *Usage    // задано на чанку Done
	Done     bool      // позначає фінальний чанк
	Raw      json.RawMessage
}
```

Драйвери задають `Usage` на чанку `Done`; його лічильники нульові, коли
провайдер не повідомив usage.

## Інструменти

Опиши викличну функцію через `Tool`, тоді прочитай виклики моделі:

```go
req := &ai.Request{
	Model:      "the-model",
	Messages:   []ai.Message{ai.UserText("Погода в Києві?")},
	Tools:      []ai.Tool{{Name: "get_weather", Description: "Поточна погода", Schema: schema}},
	ToolChoice: ai.ToolAuto,
}
resp, _ := client.Generate(ctx, req)
for _, call := range resp.ToolCalls() {
	// call.Name, call.Input (json.RawMessage), call.ID
}
```

`ToolChoice` - це `ToolAuto` (вирішує модель), `ToolNone` (заборонити виклики)
або `ToolRequired` (примусити хоча б один виклик). `Tool.Schema` - об'єкт JSON
Schema; драйвери передають його у формі, якої очікує провайдер. Щоб відповісти
на виклик, додай повідомлення `RoleTool` із `ToolResult`, чий `ID` збігається з
`ToolUse`.

## Мультимодальний ввід

`Image` - частина вмісту-зображення; подай або інлайн-байти, або URL:

```go
ai.Message{Role: ai.RoleUser, Parts: []ai.Part{
	ai.Text{Text: "Що на цьому зображенні?"},
	ai.Image{MIME: "image/png", Data: pngBytes},
}}
```

Подай інлайн `Data` з його `MIME`-типом, або `URL`, коли провайдер уміє
тягнути віддалені зображення. Драйвери кодують `Data` у base64, як вимагає їхній
формат передачі.

## Помилки

Неуспішну HTTP-відповідь нормалізовано в `*APIError`:

```go
type APIError struct {
	Status  int             // HTTP-статус
	Type    string          // тип помилки провайдера, якщо є
	Code    string          // код помилки провайдера, якщо є
	Message string          // людиночитне повідомлення, якщо є
	Raw     json.RawMessage // оригінальне тіло помилки
}
```

Розбирай через `errors.As`:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == 429 {
	// перевищено ліміт запитів
}
```

Валідація запиту повертає сентинели `ErrNoModel` і `ErrNoMessages` (звіряй через
`errors.Is`).

## Для авторів драйверів: Options і транспорт

`ai` несе плюмбінг, який перевикористовує кожен драйвер, тож драйвери лишаються
тонкими.

`Options` - спільна конфігурація, зібрана з API-ключа й функціональних опцій:

```go
o := ai.NewOptions(apiKey,
	ai.WithBaseURL("https://api.example.com"),
	ai.WithHTTPClient(httpClient),
	ai.WithTimeout(30*time.Second),
	ai.WithMaxRetries(3),
	ai.WithHeader("X-Custom", "value"),
)
```

`Options.Do` виконує HTTP-запит із ретраями та jitter-backoff, повертаючи
фінальну відповідь (зокрема останню невдалу, щоб драйвери могли прочитати тіло
помилки провайдера):

```go
resp, err := o.Do(ctx, http.MethodPost, url, body, headers)
```

`SSEEvents` читає потік Server-Sent Events як ітератор `data:`-навантажень:

```go
for data, err := range ai.SSEEvents(resp.Body) {
	// data - навантаження одної SSE-події
}
```

Ці три - `Options`, `Options.Do` і `SSEEvents` - усе, що потрібно драйверу, щоб
говорити HTTP і стрімінгом узгоджено з рештою набору.
