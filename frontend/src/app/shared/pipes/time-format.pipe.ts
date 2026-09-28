import { Pipe, PipeTransform } from '@angular/core';
import { convertTime } from '../../core/utils/utils';
@Pipe({
  name: 'timeFormat',
  standalone: true
})
export class TimeFormatPipe implements PipeTransform {
  transform(time: number): string {
    return convertTime(time);
  }
}
