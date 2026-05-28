import { Swiper, SwiperSlide } from 'swiper/react';
import { Autoplay, Pagination, A11y } from 'swiper/modules';
import 'swiper/css';
import 'swiper/css/pagination';
import type { ReactNode } from 'react';

// Themed Swiper carousel. Slides are passed as children.
export function Carousel({
  children,
  slidesPerView = 1,
  gap = 16,
  autoplay = false,
  pagination = true,
}: {
  children: ReactNode;
  slidesPerView?: number | 'auto';
  gap?: number;
  autoplay?: boolean;
  pagination?: boolean;
}) {
  const slides = Array.isArray(children) ? children : [children];
  return (
    <Swiper
      modules={[Autoplay, Pagination, A11y]}
      slidesPerView={slidesPerView}
      spaceBetween={gap}
      autoplay={autoplay ? { delay: 3200, disableOnInteraction: false } : false}
      pagination={pagination ? { clickable: true } : false}
      style={{ paddingBottom: pagination ? 32 : 0 }}
    >
      {slides.map((s, i) => (
        <SwiperSlide key={i} style={{ height: 'auto' }}>{s}</SwiperSlide>
      ))}
    </Swiper>
  );
}
