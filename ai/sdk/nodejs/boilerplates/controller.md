# Boilerplate — Controller (NestJS)

```ts
import { Body, Controller, Get, NotFoundException, Param, ParseIntPipe, Post } from '@nestjs/common';
import { {Contexto}Service, {Entidade}NotFoundError } from './{contexto}.service';
import { Create{Entidade}Dto } from './dto/create-{entidade}.dto';
import { {Entidade}ResponseDto } from './dto/{entidade}-response.dto';

@Controller('{rota}')
export class {Contexto}Controller {
  constructor(private readonly service: {Contexto}Service) {}

  @Post()
  async create(@Body() dto: Create{Entidade}Dto): Promise<{Entidade}ResponseDto> {
    return this.service.create(dto);
  }

  @Get(':id')
  async get(@Param('id', ParseIntPipe) id: number): Promise<{Entidade}ResponseDto> {
    try {
      return await this.service.get(id);
    } catch (err) {
      if (err instanceof {Entidade}NotFoundError) throw new NotFoundException(err.message);
      throw err;
    }
  }
}
```
