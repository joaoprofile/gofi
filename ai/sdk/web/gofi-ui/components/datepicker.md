# Date Picker

Seleção de data (campo + calendário em popover). Construído sobre o `Field`/`Input`
do DS — herda label, hint, erro e foco. Variantes para intervalo, data+hora e hora.

## Anatomia
`[ trigger (campo com data formatada) ] → popover [ Calendar (grade de dias) ]`.
Abre por clique/Enter; fecha por seleção, Esc ou clique fora.

## Variantes
| Componente | Para |
|-----------|------|
| `DatePicker` | uma data (ou mês, via `granularity="month"`) |
| `DateRangePicker` | intervalo (início → fim) |
| `DateTimePicker` | data + hora |
| `TimePicker` | só hora |
| `Calendar` | a grade nua (para compor inline, sem popover) |

## Props (TS) — `DatePicker`
```ts
interface DatePickerProps {
  value: Date | null;
  onChange: (date: Date) => void;
  granularity?: 'day' | 'month';
  locale?: string;                 // BCP-47, ex. 'pt-BR'
  weekStartsOn?: Weekday;
  minDate?: Date | null; maxDate?: Date | null;
  isDateDisabled?: (date: Date) => boolean;
  displayFormat?: Intl.DateTimeFormatOptions;
  placeholder?: string;
  invalid?: boolean;               // estado de erro (combine com Field)
  disabled?: boolean;
}
```

## Estados
`default · focus · open (popover) · disabled · invalid`. Datas fora de
`min/max`/`isDateDisabled` ficam desabilitadas na grade.

## a11y
- O campo tem **label** (via `Field`); o popover é um `dialog`/`grid` navegável por
  teclado (setas movem o dia, Enter seleciona, Esc fecha).
- `locale='pt-BR'` para nomes de mês/dia e formatação corretos.

## Do / Don't
- ✅ `minDate`/`maxDate`/`isDateDisabled` para restringir; ✅ `pt-BR`.
- ❌ datas só por máscara de texto sem calendário; ❌ formato ambíguo (use `locale`).

## Exemplo
```tsx
import { DatePicker } from 'gofi-ui';
import { Field } from 'gofi-ui';

<Field label="Data de início" error={erro}>
  <DatePicker value={data} onChange={setData} locale="pt-BR" minDate={hoje} />
</Field>
```
</content>
