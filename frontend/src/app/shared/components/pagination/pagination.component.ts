import {
  Component,
  input,
  output,
  computed,
  ChangeDetectionStrategy,
} from "@angular/core";
import { FontAwesomeModule } from "@fortawesome/angular-fontawesome";
import { faChevronLeft, faChevronRight } from "../../icons";
import { PaginationInfo } from "../../../core/models/song.model";

@Component({
  selector: "app-pagination",
  standalone: true,
  imports: [FontAwesomeModule],
  templateUrl: "./pagination.component.html",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PaginationComponent {
  pagination = input.required<PaginationInfo>();
  pageChange = output<number>();

  faChevronLeft = faChevronLeft;
  faChevronRight = faChevronRight;

  pageNumbers = computed(() => {
    const { currentPage, totalPages } = this.pagination();
    if (totalPages <= 7) {
      return Array.from({ length: totalPages }, (_, i) => i + 1);
    }
    const pages: (number | string)[] = [];
    if (currentPage <= 4) {
      for (let i = 1; i <= 5; i++) pages.push(i);
      pages.push("...", totalPages);
    } else if (currentPage >= totalPages - 3) {
      pages.push(1, "...");
      for (let i = totalPages - 4; i <= totalPages; i++) pages.push(i);
    } else {
      pages.push(1, "...", currentPage - 1, currentPage, currentPage + 1, "...", totalPages);
    }
    return pages;
  });

  onPageChange(page: number): void {
    const { currentPage, totalPages } = this.pagination();
    if (page !== currentPage && page >= 1 && page <= totalPages) {
      this.pageChange.emit(page);
    }
  }

  isNumber(value: number | string): value is number {
    return typeof value === "number";
  }
}
