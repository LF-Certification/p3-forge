# Test Document

This is a simple test document to verify the MkDocs static site generator. Test
it by running the following from the `mkdocs` directory: `poetry run mkdocs
serve --livereload`. This file is ignored at build time because of
`.dockerignore`.

## Formatting

- **Bold text**
- *Italic text*
- `Code snippets`
- ==This was marked (highlight)==
- ^^This was inserted (underline)^^
- ~~This was deleted (strikethrough)~~
- Subscripts: H~2~O
- Superscripts: A^T^A
- ++ctrl+alt+del++
- [Hover me](https://example.com "I'm a tooltip!")
- :material-information-outline:{ title="Important information" }

Text can be {--deleted--} and replacement text {++added++}. This can also be
combined into {~~one~>a single~~} operation. {==Highlighting==} is also
possible {>>and comments can be added inline<<}.

{==

Formatting can also be applied to blocks by putting the opening and closing
tags on separate lines and adding new lines between the tags and the content.

==}

## Buttons

[Subscribe to our newsletter](#){ .md-button }

[Subscribe to our newsletter](#){ .md-button .md-button--primary }

[Send :fontawesome-regular-envelope:](#){ .md-button }

## Admonitions (Call-outs)

!!! note "Note"
    This is a note admonition. Use it for additional information that users should be aware of.

!!! tip "Pro Tip"
    This is a tip admonition. Perfect for sharing best practices and helpful suggestions.

!!! warning "Warning"
    This is a warning admonition. Use it to alert users about potential issues or important considerations.

!!! danger "Danger"
    This is a danger admonition. Use it for critical warnings about things that could cause serious problems.

!!! success "Success"
    This is a success admonition. Great for confirming successful operations or correct implementations.

!!! question "Question"
    This is a question admonition. Useful for FAQs or highlighting common questions.

!!! info "Information"
    This is an info admonition. Use it for general informational content that stands out from regular text.

!!! example "Example"
    This is an example admonition. Perfect for providing code examples or use cases.

!!! quote "Quote"
    This is a quote admonition. Use it for citations or important quotes.

    > "The best way to predict the future is to invent it." - Alan Kay

## Code Example

### Python

```python
def hello_world():
    print("Hello, MkDocs!")
    return True
```

### JavaScript

```javascript
function helloWorld() {
    console.log("Hello, MkDocs!");
    return true;
}
```

### Go

```go
package main

import "fmt"

func helloWorld() bool {
    fmt.Println("Hello, MkDocs!")
    return true
}
```

## Code Annotations

``` yaml
theme:
  features:
    - content.code.annotate # (1)!
```

1.  :man_raising_hand: I'm a code annotation! I can contain `code`, __formatted
    text__, images, ... basically anything that can be written in Markdown.

## Code Title, Line numbers, and Highlights

``` py title="bubble_sort.py" linenums="1" hl_lines="2 4"
def bubble_sort(items):
    for i in range(len(items)):
        for j in range(len(items) - 1 - i):
            if items[j] > items[j + 1]:
                items[j], items[j + 1] = items[j + 1], items[j]
```

## Tabbed Content

=== "C"

    ``` c
    #include <stdio.h>

    int main(void) {
      printf("Hello world!\n");
      return 0;
    }
    ```

=== "C++"

    ``` c++
    #include <iostream>

    int main(void) {
      std::cout << "Hello world!" << std::endl;
      return 0;
    }
    ```

## Tables

| Method      | Description                          |
| ----------- | ------------------------------------ |
| `GET`       | :material-check:     Fetch resource  |
| `PUT`       | :material-check-all: Update resource |
| `DELETE`    | :material-close:     Delete resource |

## Definition List

`Lorem ipsum dolor sit amet`

:   Sed sagittis eleifend rutrum. Donec vitae suscipit est. Nullam tempus
    tellus non sem sollicitudin, quis rutrum leo facilisis.

`Cras arcu libero`

:   Aliquam metus eros, pretium sed nulla venenatis, faucibus auctor ex. Proin
    ut eros sed sapien ullamcorper consequat. Nunc ligula ante.

    Duis mollis est eget nibh volutpat, fermentum aliquet dui mollis.
    Nam vulputate tincidunt fringilla.
    Nullam dignissim ultrices urna non auctor.

## Task List

- [x] Lorem ipsum dolor sit amet, consectetur adipiscing elit
- [ ] Vestibulum convallis sit amet nisi a tincidunt
    * [x] In hac habitasse platea dictumst
    * [x] In scelerisque nibh non dolor mollis congue sed et metus
    * [ ] Praesent sed risus massa
- [ ] Aenean pretium efficitur erat, donec pharetra, ligula non scelerisque
