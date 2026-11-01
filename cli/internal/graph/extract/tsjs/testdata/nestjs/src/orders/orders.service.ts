import { Injectable } from '@nestjs/common';
import { Order } from './order.entity';

@Injectable()
export class OrdersService {
  create(total: number): Order {
    return { id: 'o-1', total };
  }
}
