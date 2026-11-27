# URL Shortener

A high-performance URL shortener microservice written in Go using [fasthttp](https://github.com/valyala/fasthttp) and [Redis](https://redis.io/).

---

## Table of Contents
- [Overview](#overview)
- [How It Works: Step-by-Step](#how-it-works-step-by-step)
- [The Encoding & Decoding Mechanism in Detail](#the-encoding--decoding-mechanism-in-detail)
  - [1. The Base62 Alphabet](#1-the-base62-alphabet)
  - [2. The Encoding Algorithm](#2-the-encoding-algorithm)
  - [3. The Decoding Algorithm](#3-the-decoding-algorithm)
  - [4. Mathematical Walkthrough](#4-mathematical-walkthrough)
- [Storage & Lifecycle in Redis](#storage--lifecycle-in-redis)
- [API Endpoints](#api-endpoints)
- [Getting Started](#getting-started)

---

## Overview

The service takes long, cumbersome URLs and converts them into compact, URL-safe alphanumeric tokens (e.g., `http://localhost:8080/cb`). When a user visits the shortened link, the service decodes the token back to its internal identifier, retrieves the original URL from Redis, increments an access counter, and issues an immediate HTTP 301 redirect.

---

## How It Works: Step-by-Step

### Encoding Flow (Shortening a URL)

```
Client (POST /encode/)
       │
       ▼
1. Validate URL & Expiration Date
       │
       ▼
2. Generate Random uint64 ID  ───► Check Redis if ID already exists (Collision check)
       │
       ▼
3. Store in Redis
   - Key: Shortener:<id>
   - Hash Fields: id, url, expires, visits = 0
   - Set TTL using EXPIREAT
       │
       ▼
4. Base62 Encode (uint64 ID ──► Short Code)
       │
       ▼
5. Return Short URL: http://<host>/<shortCode>
```

### Decoding & Redirection Flow (Resolving a Short URL)

```
Client (GET /<shortCode>)
       │
       ▼
1. Extract <shortCode> from request path
       │
       ▼
2. Base62 Decode (<shortCode> ──► uint64 ID)
       │
       ▼
3. Query Redis for Key: Shortener:<id>
       │
       ├─► Not Found or Expired ──► Return 404 Not Found
       │
       └─► Found:
             ├── Increment visit counter: HINCRBY Shortener:<id> visits 1
             └── Redirect client: HTTP 301 Moved Permanently (Location: original URL)
```

---

## The Encoding & Decoding Mechanism in Detail

The core of the shortening mechanism is a custom **Base62 positional numeral system** located in `base62/base62.go`.

### 1. The Base62 Alphabet

Base62 uses 62 alphanumeric characters that are safe to use in URLs without percent-encoding:

```
Index:    0-25                          26-51                         52-61
Chars:   "abcdefghijklmnopqrstuvwxyz"  "ABCDEFGHIJKLMNOPQRSTUVWXYZ"  "0123456789"
```

- Total characters ($N$): `62`
- Character mapping:
  - `'a'` = `0`, `'b'` = `1`, ..., `'z'` = `25`
  - `'A'` = `26`, `'B'` = `27`, ..., `'Z'` = `51`
  - `'0'` = `52`, `'1'` = `53`, ..., `'9'` = `61`

With 62 symbols, a 64-bit integer (`uint64`, max value $\approx 1.84 \times 10^{19}$) can be represented in at most 11 characters.

---

### 2. The Encoding Algorithm

The encoder (`base62.Encode(number uint64) string`) converts a 64-bit unsigned integer to a Base62 string using successive division and modulo operations:

```go
func Encode(number uint64) string {
    var encodedBuilder strings.Builder
    encodedBuilder.Grow(11)

    for ; number > 0; number = number / length {
        encodedBuilder.WriteByte(alphabet[(number % length)])
    }

    return encodedBuilder.String()
}
```

1. **Extract Remainder:** Compute `number % 62` to get the index of the character from the alphabet.
2. **Append Character:** Append `alphabet[remainder]` to the string.
3. **Divide:** Divide `number` by `62` (integer division).
4. **Repeat:** Repeat steps 1–3 until `number` reaches `0`.

*Note:* Because the least significant digit is appended first, the resulting string is represented in **little-endian** (least-significant digit first) order.

---

### 3. The Decoding Algorithm

The decoder (`base62.Decode(encoded string) (uint64, error)`) reconstructs the original `uint64` integer from the short code:

```go
func Decode(encoded string) (uint64, error) {
    var number uint64

    for i, symbol := range encoded {
        alphabeticPosition := strings.IndexRune(alphabet, symbol)

        if alphabeticPosition == -1 {
            return uint64(alphabeticPosition), errors.New("invalid character: " + string(symbol))
        }
        number += uint64(alphabeticPosition) * uint64(math.Pow(float64(length), float64(i)))
    }

    return number, nil
}
```

1. **Iterate Symbols:** Read each character `symbol` at position `i` (0-indexed).
2. **Alphabet Index:** Find the numeric position of `symbol` within the 62-character alphabet (`alphabeticPosition`). If the character is not in the alphabet, an invalid character error is returned.
3. **Weighted Value:** Calculate the positional weight:
   $$\text{value} = \text{alphabeticPosition} \times 62^i$$
4. **Summation:** Add the calculated value to `number`.
5. **Return Result:** The sum produces the exact original `uint64` ID.

---

### 4. Mathematical Walkthrough

#### Example: Encoding ID `125`

1. **First iteration:**
   - Modulo: $125 \pmod{62} = 1$
   - Character at index 1: `alphabet[1] = 'b'`
   - New number: $\lfloor 125 / 62 \rfloor = 2$
   - Built string: `"b"`

2. **Second iteration:**
   - Modulo: $2 \pmod{62} = 2$
   - Character at index 2: `alphabet[2] = 'c'`
   - New number: $\lfloor 2 / 62 \rfloor = 0$
   - Built string: `"bc"`

3. **Termination:** Number is 0. Output short code is `"bc"`.

#### Example: Decoding Short Code `"bc"`

1. **Character at index `i = 0` (`'b'`):**
   - Position in alphabet: $1$
   - Weight: $62^0 = 1$
   - Contribution: $1 \times 1 = 1$

2. **Character at index `i = 1` (`'c'`):**
   - Position in alphabet: $2$
   - Weight: $62^1 = 62$
   - Contribution: $2 \times 62 = 124$

3. **Total Sum:**
   $$\text{ID} = 1 + 124 = 125$$

The decoded integer `125` matches the original ID.

---

## Storage & Lifecycle in Redis

Each shortened link is stored as a **Redis Hash** under the key format:
```
Shortener:<id>
```

### Redis Schema:
| Field | Type | Description |
|---|---|---|
| `id` | `uint64` | The internal unique numerical identifier |
| `url` | `string` | The target long URL destination |
| `expires` | `string` | Human-readable expiration timestamp |
| `visits` | `int` | Counter tracking how many times the link has been clicked |

### Commands Executed:
- **Save Link:** `HMSET Shortener:<id> id <id> url <url> expires <timestamp> visits 0`
- **Set TTL:** `EXPIREAT Shortener:<id> <unix_timestamp>` (Redis automatically removes expired keys)
- **Retrieve URL & Track Visit:**
  - `HGET Shortener:<id> url`
  - `HINCRBY Shortener:<id> visits 1`
- **Inspect Metadata:** `HGETALL Shortener:<id>`

---

## API Endpoints

### 1. Create Short Link
- **POST** `/encode/`
- **Request:**
  ```json
  {
    "url": "https://github.com/valyala/fasthttp",
    "expires": "2026-12-31 23:59:59"
  }
  ```
- **Response (201 Created):**
  ```json
  {
    "success": true,
    "shortUrl": "http://localhost:8080/bc"
  }
  ```

### 2. Follow Short Link
- **GET** `/{shortLink}`
- **Response:** `301 Moved Permanently` (Redirects to target URL)

### 3. Get Short Link Info
- **GET** `/{shortLink}/info`
- **Response (200 OK):**
  ```json
  {
    "success": true,
    "shortUrl": {
      "id": 125,
      "url": "https://github.com/valyala/fasthttp",
      "expires": "2026-12-31 23:59:59.000000 +0000 UTC",
      "visits": 1
    }
  }
  ```

---

## Getting Started

### Prerequisites
- Go 1.15+
- Redis Server

### Running Locally
1. Configure `configuration.json` with your Redis host, port, and credentials.
2. Start the server:
   ```bash
   go run main.go
   ```
 
