#include <QApplication>
#include <QDateTime>
#include <QKeyEvent>
#include <QPainter>
#include <QPainterPath>
#include <QTimer>
#include <QWidget>

#include <array>
#include <cmath>

namespace {
class VitalsWindow final : public QWidget {
 public:
  VitalsWindow() {
    setWindowTitle("VAR-Studio Medical Vitals");
    setMinimumSize(1024, 600);
    setCursor(Qt::BlankCursor);
    auto *timer = new QTimer(this);
    connect(timer, &QTimer::timeout, this, [this]() {
      phase_ += 0.045;
      update();
    });
    timer->start(33);
  }

 protected:
  void paintEvent(QPaintEvent *) override {
    QPainter painter(this);
    painter.setRenderHint(QPainter::Antialiasing);
    painter.fillRect(rect(), QColor("#081310"));
    drawHeader(painter);
    drawWavePanel(painter);
    drawMetrics(painter);
    drawFooter(painter);
  }

  void keyPressEvent(QKeyEvent *event) override {
    if (event->key() == Qt::Key_Escape) {
      close();
    }
  }

 private:
  void drawHeader(QPainter &painter) {
    painter.setPen(QColor("#ecf8f2"));
    QFont title = painter.font();
    title.setPixelSize(28);
    title.setBold(true);
    painter.setFont(title);
    painter.drawText(42, 55, "Patient Monitor");
    painter.setPen(QColor("#7f9d91"));
    QFont small = painter.font();
    small.setPixelSize(13);
    small.setBold(false);
    painter.setFont(small);
    painter.drawText(
        42,
        80,
        "VAR-Studio · simulated clinical data");
    painter.setPen(QColor("#65e6ab"));
    painter.drawText(width() - 245, 55, "●  MONITORING");
    painter.setPen(QColor("#7f9d91"));
    painter.drawText(
        width() - 245,
        80,
        QDateTime::currentDateTime().toString("hh:mm:ss"));
  }

  void drawGrid(QPainter &painter, const QRectF &area) {
    painter.save();
    painter.setClipRect(area);
    painter.setPen(QPen(QColor(34, 69, 58, 90), 1));
    for (int x = int(area.left()); x < area.right(); x += 32) {
      painter.drawLine(x, int(area.top()), x, int(area.bottom()));
    }
    for (int y = int(area.top()); y < area.bottom(); y += 32) {
      painter.drawLine(int(area.left()), y, int(area.right()), y);
    }
    painter.restore();
  }

  double pulse(double value) const {
    const double beat = std::fmod(value, 1.0);
    if (beat < 0.04) return beat * 5.0;
    if (beat < 0.08) return 0.2 - (beat - 0.04) * 25.0;
    if (beat < 0.12) return -0.8 + (beat - 0.08) * 45.0;
    if (beat < 0.17) return 1.0 - (beat - 0.12) * 24.0;
    if (beat < 0.22) return -0.2 + (beat - 0.17) * 4.0;
    return 0.02 * std::sin(value * 18.0);
  }

  void drawWave(
      QPainter &painter,
      const QRectF &area,
      const QColor &color,
      double offset,
      bool ecg) {
    QPainterPath path;
    for (int x = 0; x <= int(area.width()); x += 2) {
      const double t = phase_ + offset + x / 170.0;
      const double sample = ecg
          ? pulse(t)
          : std::sin(t * 3.2) * 0.46 +
              std::sin(t * 1.1) * 0.12;
      const double y =
          area.center().y() - sample * area.height() * 0.35;
      if (x == 0) {
        path.moveTo(area.left(), y);
      } else {
        path.lineTo(area.left() + x, y);
      }
    }
    painter.setPen(QPen(color, 2.4));
    painter.drawPath(path);
  }

  void drawWavePanel(QPainter &painter) {
    const QRectF panel(40, 112, width() * 0.64, height() - 180);
    painter.setPen(QColor("#17362d"));
    painter.setBrush(QColor("#0c1b17"));
    painter.drawRoundedRect(panel, 14, 14);
    drawGrid(painter, panel.adjusted(18, 20, -18, -20));
    const qreal row = panel.height() / 3.0;
    const std::array<QColor, 3> colors = {
        QColor("#57e5a3"),
        QColor("#52b8ff"),
        QColor("#ffcc62"),
    };
    const std::array<QString, 3> labels = {
        "ECG",
        "SpO₂",
        "RESP",
    };
    for (int i = 0; i < 3; ++i) {
      const QRectF area(
          panel.left() + 20,
          panel.top() + i * row + 20,
          panel.width() - 40,
          row - 38);
      painter.setPen(colors[i]);
      painter.drawText(area.left(), area.top() + 12, labels[i]);
      drawWave(
          painter,
          area.adjusted(60, 0, 0, 0),
          colors[i],
          i,
          i == 0);
    }
  }

  void metric(
      QPainter &painter,
      const QRectF &area,
      const QString &label,
      const QString &value,
      const QString &unit,
      const QColor &color) {
    painter.setPen(QColor("#17362d"));
    painter.setBrush(QColor("#0c1b17"));
    painter.drawRoundedRect(area, 14, 14);
    painter.setPen(QColor("#8da99e"));
    QFont font = painter.font();
    font.setPixelSize(13);
    font.setBold(true);
    painter.setFont(font);
    painter.drawText(
        area.adjusted(20, 17, 0, 0),
        Qt::AlignLeft | Qt::AlignTop,
        label);
    font.setPixelSize(43);
    painter.setFont(font);
    painter.setPen(color);
    painter.drawText(
        area.adjusted(20, 45, 0, 0),
        Qt::AlignLeft | Qt::AlignTop,
        value);
    font.setPixelSize(12);
    font.setBold(false);
    painter.setFont(font);
    painter.drawText(
        area.adjusted(22, 100, 0, 0),
        Qt::AlignLeft | Qt::AlignTop,
        unit);
  }

  void drawMetrics(QPainter &painter) {
    const qreal left = width() * 0.68;
    const qreal cardWidth = width() - left - 40;
    const qreal gap = 12;
    const qreal cardHeight = (height() - 204) / 3.0;
    const int heartRate = 72 + int(std::sin(phase_) * 2.0);
    metric(
        painter,
        QRectF(left, 112, cardWidth, cardHeight),
        "HEART RATE",
        QString::number(heartRate),
        "BPM",
        QColor("#57e5a3"));
    metric(
        painter,
        QRectF(
            left,
            112 + cardHeight + gap,
            cardWidth,
            cardHeight),
        "OXYGEN SATURATION",
        "98",
        "% SpO₂",
        QColor("#52b8ff"));
    metric(
        painter,
        QRectF(
            left,
            112 + 2 * (cardHeight + gap),
            cardWidth,
            cardHeight),
        "BLOOD PRESSURE",
        "120/80",
        "mmHg",
        QColor("#ffcc62"));
  }

  void drawFooter(QPainter &painter) {
    painter.setPen(QColor("#668178"));
    const QRect area(42, height() - 42, width() - 84, 24);
    painter.drawText(
        area,
        Qt::AlignLeft,
        "DEMO MODE · Simulated values · Not for medical use");
    painter.drawText(area, Qt::AlignRight, "Press Esc to close");
  }

  double phase_ = 0.0;
};
}  // namespace

int main(int argc, char *argv[]) {
  QApplication app(argc, argv);
  VitalsWindow window;
  window.showFullScreen();
  return app.exec();
}
