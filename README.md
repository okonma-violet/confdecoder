# Confdecoder
Читает файл. Декодит прочитанное.
## Вид записи
><em><strong># comment</strong></em>\
><em><strong>key</strong></em>\
><em><strong>key</strong> value</em>\
><em><strong>key</strong> value value value</em>
## Поддерживаемые форматы данных <em>value</em>
<ul>
  <li><em><strong>string</em></strong>
    <li><strong><em>string</em></strong> slice
      <li><strong><em>int</em></strong>
        <li><strong><em>int</em></strong> slice
          <li>nested <strong><em>struct</em></strong>
            <li> pointer to <strong><em>string | int | struct</em></strong>
</ul>

## Ключи
<ul>
  <li>Ключи пишутся в слайс string отдельно, в хронологическом порядке
  <li>При повторении ключа дубликат ключа отбрасывается, не влияя на хронологический порядок
  <li>При повторении ключа легитимно последнее его значение
  <li>Ключи пишутся даже при отсутствии значения для оных
