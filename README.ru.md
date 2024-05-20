# tcontainer

[![en](https://img.shields.io/badge/lang-en-red.svg)](README.md)

Обёртка над https://github.com/ory/dockertest

Предоставляет дополнительные удобства для создания docker контейнеров в тестах:
- более удобный синтаксис создания контейнеров через использование опций
- возможность переиспользовать контейнер если он уже существует `WithReuseContainer(...)`
- возможность удалить старый контейнер при создании нового вместо получения ошибки `docker.ErrContainerAlreadyExists`

## Пример использования

```go
package main

import (
	"errors"
	"time"

	"github.com/kiteggrad/tcontainer"
	"github.com/ory/dockertest/v3"
)

func main() {
	connectToDB := func(host string, port int, user, password string) (err error) {
		_, _, _, _ = host, port, user, password
		return errors.New("unimplemented")
	}

	dockerPool, container, err := tcontainer.New(
		"postgres",
		tcontainer.WithImageTag("latest"),
		tcontainer.WithRetry(
			func(container *dockertest.Resource, apiEndpoints map[int]tcontainer.ApiEndpoint) (err error) {
				return connectToDB(apiEndpoints[5432].IP, apiEndpoints[5432].Port, "user", "pass")
			},
			0, // use default retry timeout
		),
		tcontainer.WithReuseContainer(true, 0, true), // reuseContainer, reuseTimeout, recreateOnError
		tcontainer.WithAutoremove(false),
		tcontainer.WithExpiry(time.Minute*10),
	)
	if err != nil {
		panic(err)
	}
	defer func() { _ = container.Close() }()

	_ = dockerPool
	_ = container
}
```

## Список опций
- ### `WithImageTag`
    Позволяет задать свой тэг для образа

    По умолчанию: `latest`

    ```go
    tcontainer.WithImageTag("v1.0.0")
    ```
- ### `WithContainerName`
    Позволяет задать своё имя для контейнера

    По умолчанию: docker создаёт случайное имя

    ```go
    tcontainer.WithContainerName("test-postgres")
    // or
    tcontainer.WithContainerName(t.Name()+"-pg")
    ```
- ### `WithENV`
    Позволяет передать набор env переменных в контейнер

    ```go
    tcontainer.WithENV("USER=root", "PASSWORD=password")
    ```
- ### `WithCMD`
    Позволяет задать команду для старта контейнера

    По умолчанию: выполняется указанный в Dockerfile образа

    ```go
    tcontainer.WithCMD("sh", "-c", "server start")
    ```
- ### `WithRetry`
    Позволяет задать команду проверяющую что контейнер успешно запущен и готов к работе.
    - Функции `New` / `NewWithPool` будут периодически запускать и ожидать успешного завершения `retryOperation`
    или выдавать ошибку при достижении `retryTimeout`.  
    - `apiEndpoints` позволяет получить доступный извне ip и port для подключения к конкретному внутреннему порту контейнера.

    По умолчанию: 
    - `retryOperation` не выполняется, функции `New` / `NewWithPool` завершаются сразу после создания контейнера 
    - `retryTimeout` - `time.Minute`

    ```go
    tcontainer.WithRetry(
        func(container *dockertest.Resource, apiEndpoints map[int]tcontainer.ApiEndpoint) (err error) {
            return connectToDB(apiEndpoints[5432].IP, apiEndpoints[5432].Port, "user", "pass")
        },
        0, // use default retry timeout
	)
    ```
- ### `WithExposedPorts`
    Позволяет задать порты для публикации. Аналогично EXPOSE в Dockerfile

    На текущий момент все указанные порты будут публиковаться в tcp (8080/tcp). <!-- //TODO: implement -->
    Возможно в дальнейшем буждет добавлена возможность задать другой протокол.

    По умолчанию: не публикуется

    ```go
    tcontainer.WithExposedPorts(8080, 8081)
    ```
- ### `WithPortBindings`
    Позволяет задать привязку приватных портов к конкретным публичным - `map[privatePort]publicPort`

    На текущий момент все указанные порты будут рассматриваться в tcp (8080/tcp). <!-- //TODO: implement -->
    Возможно в дальнейшем буждет добавлена возможность задать другой протокол.

    По умолчанию: все публичные порты генерируются случайными

    ```go
    tcontainer.WithPortBindings(map[int]int{80: 8080, 443: 8443})
    ```
- ### `WithExpiry`
    Позволяет задать время после которого контейнер будет остановлен.
    Можно задать пустое значение, тогда контейнер не будет остановлен через какое-либо время.

    По умолчанию: time.Minute

    ```go
    tcontainer.WithExpiry(time.Minute * 10)
    // or
    tcontainer.WithExpiry(0)
    ```
- ### `WithAutoremove`
    Позволяет задать будет ли контейнер удалён сразу после остановки (в том числе по expiry).

    По умолчанию: `true`

    ```go
    tcontainer.WithAutoremove(false)
    ```
- ### `WithReuseContainer`
    Позволяет переиспользовать контейнер вместо получения ошибки о том что контейнер уже существует.
    - Не должен использоваться вместе с `WithRemoveContainerOnExists` - вернёт ошибку `ErrOptionsConflict`.
    - Вы можете получить ошибку если уже существующий контейнер имеет другие настройки (другой маппинг портов или имя образа). Эта ошибка может быть проигнорирована при `recreateOnError`
    - Можно задать `reuseTimeout` чтобы изменить таймаут ожидания пока старый контейнер не снимется с паузы или не запустится.
    - Можно задать `recreateOnError` чтобы пересоздать контейнер вместо получения ошибки при попытке его переиспользовать. - Когда старый контейнер имеет другие настройки или не получилось его реанимировать

    По умолчанию: 
    - `reuseContainer` - `false`
    - `reuseTimeout` - `time.Minute`
    - `recreateOnError` - `false`

    ```go
    tcontainer.WithReuseContainer(true, 0, true)
    ```
- ### `WithRemoveContainerOnExists`
    Позволяет удалить существующий контейнер вместо получения ошибки о том что контейнер уже существует.
    - Не должен использоваться вместе с `WithReuseContainer` - вернёт ошибку `ErrOptionsConflict`.

    По умолчанию: `false`

    ```go
    tcontainer.WithRemoveContainerOnExists(true)
    ```
