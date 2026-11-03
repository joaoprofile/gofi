# Boilerplate — DTO (NestJS)

```ts
import { Type } from 'class-transformer';
import { IsIn, IsInt, Min } from 'class-validator';

export class Consulta{Entidade}Dto {
  @Type(() => Number)
  @IsInt()
  @Min(0)
  valorCentavos: number;

  @IsIn(['a', 'b', 'c'])
  nivel: string;
}
```
