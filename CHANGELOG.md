## 1.0.2

- Fix merging of boolean values in YAML dictionaries

## 1.0.1

- Fix error handling in `yaml_merge` provider function

## 1.0.0

- BREAKING CHANGE: Do not deduplicate items in a list of primitive values, for example a list of strings
- BREAKING CHANGE: Deduplicate items in a list of dictionaries consistently, regardless of whether they are in the same YAML string or not

## 0.2.6

- Add `yaml_merge` provider function

## 0.2.5

- Do not overwrite existing values with empty/null values

## 0.2.4

- Fix deadlock when resolving environment variables

## 0.2.3

- Fix crash due to concurrency issue

## 0.2.2

- Do not merge YAML dictionary list items, where each list item has unique attributes with primitive values

## 0.2.1

- Add support for YAML `!env` tags to resolve values from environment variables

## 0.2.0

- Append matching primitive list items if `merge_list_items` set to `false`
- Deep merge list items with extra primitive values if `merge_list_items` set to `true`

## 0.1.4

- Fix default value of `merge_list_items` attribute

## 0.1.3

- Introduce `merge_list_items` flag (default value is `true`) to disable merging of list items

## 0.1.2

- Fix merging of non-string type lists

## 0.1.1

- Fix list item value comparison

## 0.1.0

- Initial release
