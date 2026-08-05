# Ldapper — дизайн v1

Дата: 2026-08-05
Статус: утверждён

## 1. Что это

Десктопный обозреватель LDAP-каталогов — замена Sysinternals AD Explorer. Работает с Active
Directory и с обычными LDAP-серверами (OpenLDAP, FreeIPA, 389 Directory Server). Первая версия
только читает: обход дерева, просмотр атрибутов, поиск по фильтрам, экспорт.

Ключевое отличие от `ldapsearch` и веб-панелей: сырое значение атрибута всегда показано, но рядом
с ним стоит расшифровка. `objectSid` в base64 бесполезен, `S-1-5-21-…-1104` — нет.

## 2. Стек и платформы

| Слой | Решение |
|---|---|
| Ядро | Go, `github.com/go-ldap/ldap/v3` |
| Оболочка | Wails |
| Фронтенд | React + TypeScript + Vite |
| Стили | CSS custom properties по дизайн-системе Superlist |
| Хранение секретов | `github.com/zalando/go-keyring` |

Приоритет платформ: Windows — основная, macOS — вторая, Linux — по возможности. Сборка всех трёх
в CI, но требования к качеству предъявляются к Windows.

Windows требует WebView2 Runtime. Установщик поставляется в варианте с бандлом рантайма.

## 3. Архитектура

Go-ядро и React общаются только через фасад в `app/`. Фасад переводит доменные типы в DTO и не
содержит логики — вся логика живёт в пакетах ниже и покрыта тестами без участия Wails.

```
ldapper/
  main.go                  точка входа Wails
  app/                     фасад: методы, привязанные к фронтенду
  internal/
    session/               дозвон, TLS, bind, переподключение, отмена
    browse/                листинг одного уровня, постраничность
    search/                выполнение поиска, стриминг результатов
    schema/                RootDSE, subschemaSubentry, синтаксисы атрибутов
    decode/                расшифровка значений (SID, GUID, FILETIME, битовые флаги)
    filters/               библиотека фильтров: встроенные, правки, подстановки
    profiles/              профили подключений, работа с keychain
    export/                LDIF и CSV
  frontend/                React + Vite
  docs/design/             мокапы
  build/                   иконки, NSIS, entitlements
```

### 3.1 session

Отвечает за жизненный цикл соединения.

- Схемы: `ldap://` (открытый), `ldaps://` (порт 636), `ldap://` + StartTLS.
- Bind: simple (DN или UPN + пароль) и NTLM (`ldap.NTLMBind`, домен + логин + пароль).
- Ошибку проверки сертификата не глотает: возвращает типизированную `ErrUntrustedCert` с
  отпечатком SHA-256, subject, issuer и сроком действия. Решение доверять принимается
  пользователем и записывается в конкретный профиль подключения, не глобально.
- Одно активное соединение на профиль, несколько профилей одновременно.
- Каждый запрос принимает `context.Context`; закрытие вкладки отменяет запрос.
- Разрыв связи: одна автоматическая попытка переподключения с тем же bind, дальше — ошибка в UI.

### 3.2 browse

Листинг детей узла: `scope = one level`, страницами через контрол `1.2.840.113556.1.4.319`.
Размер страницы по умолчанию 1000 — предел AD по умолчанию. Возвращает курсор, по которому UI
догружает следующую страницу. Число подчинённых объектов берётся, если сервер отдаёт
`numSubordinates`; иначе счётчик не показывается.

### 3.3 search

Принимает сырой фильтр RFC 4515, scope и base DN. Валидирует фильтр через `ldap.CompileFilter`
до отправки. Результаты не копятся в памяти целиком — отдаются пачками через события Wails, UI
наполняет таблицу на ходу. Поиск прерывается кнопкой Stop.

`sizeLimitExceeded` и `timeLimitExceeded` — не ошибки, а состояние «показано не всё»: результаты
остаются, в статус-баре появляется предупреждение.

### 3.4 schema

При подключении читает RootDSE: `supportedControl`, `supportedSASLMechanisms`, `namingContexts`,
`vendorName`, `subschemaSubentry`. По наличию `1.2.840.113556.1.4.800` определяет, что это
Active Directory — от этого зависят набор доступных фильтров и применяемые декодеры.

Схема (`attributeTypes`, `objectClasses`) читается лениво, при первом открытии объекта, и
кэшируется на время сессии. Из неё берутся синтаксис атрибута и признак единственности значения.

### 3.5 decode

Чистые функции без сети — основная масса юнит-тестов.

| Атрибут / синтаксис | Во что превращается |
|---|---|
| `objectSid` | `S-1-5-21-3638186439-3476304828-32309524-1104` |
| `objectGUID` | `a3f19b0c-4d2e-4f1a-9c88-0b7e5d2a41f6` |
| `pwdLastSet`, `lastLogonTimestamp`, `accountExpires` | дата UTC; `0` и `0x7FFFFFFFFFFFFFFF` — особые значения |
| `userAccountControl` | список флагов: `NORMAL_ACCOUNT`, `DONT_EXPIRE_PASSWORD`, `ACCOUNTDISABLE`, … |
| `groupType` | тип и область действия группы |
| `sAMAccountType` | человекочитаемый тип учётной записи |
| `GeneralizedTime` | дата UTC |
| нераспознанный OctetString | hex + base64 |

Декодер, не сумевший разобрать значение, обязан вернуть сырое значение, а не ошибку. Показ
данных не должен ломаться из-за неожиданного содержимого каталога.

### 3.6 filters

Библиотека фильтров: 18 встроенных плюс пользовательские.

**Модель записи:**

```json
{
  "id": "ad-stale-users-90d",
  "name": "Stale users · 90 days",
  "description": "Enabled accounts that haven't authenticated in 90 days.",
  "filter": "(&(objectCategory=person)(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(lastLogonTimestamp<={{now-90d:filetime}}))",
  "scope": "subtree",
  "base": "",
  "columns": ["cn", "sAMAccountName", "lastLogonTimestamp", "distinguishedName"],
  "dialect": "ad"
}
```

`dialect` принимает `ad`, `generic` или `posix`. Фильтр, чей диалект сервер не поддерживает,
показывается приглушённым и не запускается — вместо тихого пустого результата пользователь
получает объяснение.

`base` пустой означает «от корня активного naming context».

**Подстановки.** Половина полезных фильтров привязана ко времени, поэтому фильтр — шаблон, а не
константа:

| Выражение | Разворачивается в |
|---|---|
| `{{now:filetime}}` | 18-значный Windows FILETIME |
| `{{now-90d:filetime}}` | то же, минус 90 дней |
| `{{now-6h:generalized}}` | `20260805T…Z` в формате GeneralizedTime |
| `{{me}}` | DN учётной записи, под которой выполнен bind |

Единицы: `d`, `h`, `m`. Развёрнутое значение показывается под полем ввода до запуска —
пользователь видит, что реально уйдёт на сервер.

**Хранение и правки.** Встроенный набор вкомпилирован в бинарник через `go:embed`. Правки
лежат отдельно, в `filters.json` в конфиге ОС:

- Windows: `%APPDATA%\Ldapper\filters.json`
- macOS: `~/Library/Application Support/Ldapper/filters.json`
- Linux: `$XDG_CONFIG_HOME/ldapper/filters.json`

Файл содержит три секции: `overrides` (правки встроенных, ключ — `id`), `custom`
(пользовательские) и `removed` (список `id` удалённых встроенных — чтобы они не возвращались).

Правка встроенного фильтра пишет запись в `overrides`; в списке он помечается коралловой точкой.
`Reset` удаляет запись — фильтр возвращается к заводскому виду. `Restore all defaults` очищает
`overrides` и `removed`, не трогая `custom`.

**Встроенный набор.**

Только Active Directory (12):

| Имя | Фильтр |
|---|---|
| Disabled accounts | `(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=2))` |
| Locked out right now | `(&(objectCategory=person)(objectClass=user)(lockoutTime>=1))` |
| Password never expires | `(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=65536))` |
| Must change at next logon | `(&(objectCategory=person)(objectClass=user)(pwdLastSet=0))` |
| Stale users · 90 days | `(&(objectCategory=person)(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(lastLogonTimestamp<={{now-90d:filetime}}))` |
| Stale computers · 60 days | `(&(objectCategory=computer)(lastLogonTimestamp<={{now-60d:filetime}}))` |
| Privileged accounts | `(&(objectCategory=person)(objectClass=user)(adminCount=1))` |
| Accounts with an SPN | `(&(objectCategory=person)(objectClass=user)(servicePrincipalName=*))` |
| Kerberos pre-auth disabled | `(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))` |
| Unconstrained delegation | `(&(objectCategory=computer)(userAccountControl:1.2.840.113556.1.4.803:=524288))` |
| Domain controllers | `(&(objectCategory=computer)(userAccountControl:1.2.840.113556.1.4.803:=8192))` |
| Group policy objects | `(objectClass=groupPolicyContainer)` |

Любой каталог (6):

| Имя | Фильтр |
|---|---|
| All people | `(\|(objectClass=person)(objectClass=inetOrgPerson))` |
| All groups | `(\|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))` |
| All organizational units | `(objectClass=organizationalUnit)` |
| Empty groups | `(&(\|(objectClass=group)(objectClass=groupOfNames))(!(member=*)))` |
| People without email | `(&(objectClass=person)(!(mail=*)))` |
| POSIX accounts | `(objectClass=posixAccount)` — диалект `posix` |

### 3.7 profiles

Профиль подключения: имя, хост, порт, шифрование, способ bind, логин, флаг «помнить пароль»,
доверенные отпечатки сертификатов. Хранится в `connections.json` рядом с фильтрами.

Пароль в JSON не попадает никогда. При включённом флаге он уходит в хранилище ключей ОС
(Windows Credential Manager, Keychain, Secret Service) под ключом `ldapper:<profile-id>`.
Недоступное хранилище — не фатальная ошибка: пароль просто спрашивается каждый раз, о чём
сказано в интерфейсе.

### 3.8 export

LDIF по RFC 2849 (значения не в ASCII — base64 с `::`) и CSV с выбранным набором колонок.
Экспортируется то, что видно: выделенный объект, поддерево или результаты поиска. Пишется
потоком в файл, без сборки всего в памяти.

## 4. Интерфейс

Дизайн-система — Superlist (`docs/design/mockup-v1-superlist.html`).

| Токен | Значение | Роль |
|---|---|---|
| Aubergine Canvas | `#181824` | холст окна |
| Well | `#1f1e30` | панели, поля ввода |
| Elevated Plum | `#26253b` | корпус окна, модальные |
| Raise | `#2f2e47` | расшифровки, активная вкладка, подсветка совпадений |
| Coral Ember | `#ff4a36` | ровно один элемент на экран |
| Iris Accent | `#535676` | иконки в покое, счётчики |
| Pure White | `#ffffff` | заголовки, значения атрибутов |
| Ash / Fog | `#8e8da0` / `#696f81` | текст интерфейса, сырые значения |

Правило бихроматики соблюдается строго: третьего акцентного цвета нет. Коралл на каждом экране
занят ровно одной ролью — выделенная строка, либо основное действие, либо предупреждение.
Скругления: карточки 20px, панели 16px, поля 8px, кнопки 100px. Теней-подсветок нет — обычные
мягкие тени.

Язык интерфейса — английский. Имена атрибутов и классов непереводимы, аудитория работает в
англоязычной консоли.

Шрифты: Inter Tight для заголовков и интерфейса, системный моноширинный для DN и значений.
Оригинальные Haffer XH и Jersey 10 — коммерческие, в проект не берём.

**Экраны:**

1. **Обзор** — дерево слева, атрибуты справа. Виртуализация обеих панелей обязательна с первого
   дня: OU на 50 000 объектов — норма. Постраничная догрузка видна пользователю строкой
   `loading page 2 of 4`, а не бесконечным спиннером.
2. **Поиск** — сырой фильтр в строке, результаты потоком, счётчик найденного и просканированного.
   Клик по строке раскрывает объект в дереве.
3. **Библиотека фильтров** — список с метками диалекта и редактор: имя, фильтр с подсветкой,
   scope, база, колонки результата, описание. Кнопки Save, Reset, Duplicate, Delete.
4. **Подключение** — хост, порт, шифрование, способ входа, учётные данные.
5. **Недоверенный сертификат** — отпечаток SHA-256, subject, issuer, срок. Кнопка подтверждения
   намеренно не окрашена в акцентный цвет: продолжить вопреки предупреждению не должно выглядеть
   рекомендованным действием.

## 5. Ошибки

Коды LDAP переводятся в текст, объясняющий причину и следующий шаг:

| Код | Текст |
|---|---|
| `invalidCredentials` (49) | неверный логин или пароль; для AD дополнительно разбирается подкод `data 525/52e/530/531/532/533/701/773/775` |
| `insufficientAccessRights` (50) | учётной записи не хватает прав на этот объект |
| `sizeLimitExceeded` (4) | показана часть результатов, сервер оборвал выдачу |
| `referral` (10) | объект живёт в другом naming context, с предложением подключиться туда |
| `unwillingToPerform` (53) | сервер отказал — обычно требование TLS до bind |

Сетевые и TLS-ошибки не смешиваются с ошибками протокола: первые ведут к экрану подключения,
вторые показываются в контексте операции.

## 6. Тестирование

- `decode`, `export`, `filters` (разбор шаблонов и валидация) — юнит-тесты, без сети. Основное
  покрытие здесь.
- `session`, `browse`, `search` — интеграционные тесты за build-тегом `integration` против
  OpenLDAP в Docker с заранее загруженным LDIF, плюс Samba AD DC для проверки AD-специфики:
  постраничность, правило сравнения `1.2.840.113556.1.4.803`, NTLM bind.
- Фронтенд — vitest на форматтеры и разбор шаблонов подстановки. E2E в v1 нет.
- Тесты пишутся по-английски.

## 7. Границы v1

Не входит: любая запись в каталог, снапшоты и сравнение состояний, редактирование схемы,
Kerberos и GSSAPI, поиск контроллеров домена по SRV-записям, разбор `nTSecurityDescriptor`,
конструктор фильтров мышью, локализация интерфейса.

Снапшоты и diff — главный кандидат в v2. Архитектура это учитывает: `browse` и `search`
возвращают данные, а не рисуют их, поэтому обход поддерева для снятия слепка переиспользует
существующий код без переделки.
