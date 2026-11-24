import React, { useState, useEffect } from 'react';
import {
  ShoppingBag,
  BookOpen,
  Award,
  QrCode,
  CheckCircle2,
  AlertTriangle,
  ArrowLeft,
  X,
  CreditCard,
  Download,
  Clock,
  Sparkles,
} from 'lucide-react';
import { apiClient } from '../../api/client';
import { Product, ShopOrder, PaymentSimulationResult } from '../../types';

interface ShopViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

export const ShopView: React.FC<ShopViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedCategory, setSelectedCategory] = useState<'all' | 'digital' | 'subscription'>('all');

  // Checkout modal state
  const [activeOrder, setActiveOrder] = useState<ShopOrder | null>(null);
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [simResult, setSimResult] = useState<PaymentSimulationResult | null>(null);

  // Fetch catalog
  useEffect(() => {
    loadCatalog();
  }, []);

  const loadCatalog = async () => {
    try {
      setLoading(true);
      const res = await apiClient.getShopCatalog();
      setProducts(res.products || []);
    } catch (err: any) {
      onErrorToast('Не удалось загрузить каталог товаров');
      // Fallback default demo products if backend unreachable
      setProducts([
        {
          id: 'prod_guide',
          title: '📘 PDF Гайд по разработке ботов',
          description: 'Практическое руководство по созданию масштабируемых ботов и Mini App в MAX и Telegram.',
          price: 490,
          category: 'digital',
          item_type: 'digital',
          content_payload: 'https://example.com/downloads/bot-guide.pdf',
          is_active: true,
        },
        {
          id: 'prod_sub_vip',
          title: '⭐ VIP Подписка на закрытый клуб (1 мес)',
          description: 'Доступ к базе готовых шаблонов, исходникам ботов и закрытому сообществу архитекторов.',
          price: 1490,
          category: 'subscription',
          item_type: 'subscription',
          content_payload: 'https://t.me/+exclusive_bothub_vip_invite',
          is_active: true,
        },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const handleBuy = async (prod: Product) => {
    try {
      setIsProcessing(true);
      setSelectedProduct(prod);
      setSimResult(null);

      const res = await apiClient.createShopOrder(prod.id);
      setActiveOrder(res.order);
    } catch (err: any) {
      // Fallback local mock order
      const mockOrder: ShopOrder = {
        id: 'ord_' + Math.random().toString(36).substring(2, 9),
        user_id: 1,
        product_id: prod.id,
        status: 'pending',
        qr_code: `demo:pay:${prod.id}:${prod.price}rub`,
        created_at: new Date().toISOString(),
      };
      setActiveOrder(mockOrder);
    } finally {
      setIsProcessing(false);
    }
  };

  const handleSimulate = async (action: 'paid' | 'abandoned') => {
    if (!activeOrder) return;
    try {
      setIsProcessing(true);
      const res = await apiClient.simulateShopPayment(activeOrder.id, action);
      setSimResult(res.result);
      if (action === 'paid') {
        onSuccessToast('Оплата подтверждена! Товар доставлен.');
      } else {
        onErrorToast('Заказ переведен в статус "Оставлен". Напоминание отправлено.');
      }
    } catch (err: any) {
      // Local fallback
      setSimResult({
        order_id: activeOrder.id,
        status: action,
        message:
          action === 'paid'
            ? 'Оплата успешно завершена (симуляция)'
            : 'Заказ отменен / брошен (симуляция)',
        delivery_payload:
          action === 'paid'
            ? selectedProduct?.content_payload || 'https://example.com/delivery'
            : undefined,
        reminder_scheduled: action === 'abandoned',
      });
    } finally {
      setIsProcessing(false);
    }
  };

  const handleCloseModal = () => {
    setActiveOrder(null);
    setSelectedProduct(null);
    setSimResult(null);
  };

  const filteredProducts = products.filter((p) => {
    if (selectedCategory === 'all') return true;
    return p.item_type === selectedCategory;
  });

  return (
    <div className="w-full max-w-2xl mx-auto px-4 py-4 space-y-5 animate-in fade-in duration-200">
      {/* Header with Back button */}
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 border border-slate-800 text-xs font-medium text-slate-300 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>К списку режимов</span>
        </button>

        <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 text-xs font-semibold">
          <ShoppingBag className="w-3.5 h-3.5" />
          <span>Режим: Магазин</span>
        </div>
      </div>

      {/* Banner / Title */}
      <div className="p-4 rounded-2xl bg-gradient-to-r from-indigo-950/60 via-slate-900 to-slate-900 border border-indigo-900/50">
        <h2 className="text-lg font-bold text-white flex items-center gap-2">
          <span>Витрина цифровых товаров и подписок</span>
        </h2>
        <p className="text-xs text-slate-400 mt-1">
          Демонстрация каталога, моментальной генерации платежного QR-кода и сценариев выдачи контента при оплате либо дожима брошенных корзин.
        </p>
      </div>

      {/* Category Tabs */}
      <div className="flex items-center gap-2 p-1 rounded-xl bg-slate-900 border border-slate-800">
        <button
          onClick={() => setSelectedCategory('all')}
          className={`flex-1 py-1.5 text-xs font-medium rounded-lg transition-all ${
            selectedCategory === 'all'
              ? 'bg-indigo-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          Все товары
        </button>
        <button
          onClick={() => setSelectedCategory('digital')}
          className={`flex-1 flex items-center justify-center gap-1.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            selectedCategory === 'digital'
              ? 'bg-indigo-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <BookOpen className="w-3.5 h-3.5" />
          <span>Цифровые гайды</span>
        </button>
        <button
          onClick={() => setSelectedCategory('subscription')}
          className={`flex-1 flex items-center justify-center gap-1.5 py-1.5 text-xs font-medium rounded-lg transition-all ${
            selectedCategory === 'subscription'
              ? 'bg-indigo-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Award className="w-3.5 h-3.5" />
          <span>Платные подписки</span>
        </button>
      </div>

      {/* Product List */}
      {loading ? (
        <div className="py-12 text-center text-slate-400 text-sm">
          Загрузка каталога...
        </div>
      ) : filteredProducts.length === 0 ? (
        <div className="py-12 text-center text-slate-400 text-sm">
          В данной категории пока нет товаров.
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          {filteredProducts.map((prod) => (
            <div
              key={prod.id}
              className="flex flex-col justify-between p-4 rounded-2xl bg-slate-900/90 border border-slate-800 hover:border-slate-700 transition-all shadow-md shadow-slate-950/40"
            >
              <div>
                <div className="flex items-center justify-between mb-2">
                  <span
                    className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
                      prod.item_type === 'subscription'
                        ? 'bg-purple-500/10 text-purple-400 border border-purple-500/20'
                        : 'bg-sky-500/10 text-sky-400 border border-sky-500/20'
                    }`}
                  >
                    {prod.item_type === 'subscription' ? 'Подписка' : 'Цифровой товар'}
                  </span>
                  <span className="text-sm font-bold text-white">
                    {prod.price.toLocaleString('ru-RU')} ₽
                  </span>
                </div>

                <h3 className="font-semibold text-sm text-slate-100 mb-1 leading-snug">
                  {prod.title}
                </h3>
                <p className="text-xs text-slate-400 leading-relaxed mb-4">
                  {prod.description}
                </p>
              </div>

              <button
                onClick={() => handleBuy(prod)}
                disabled={isProcessing}
                className="w-full flex items-center justify-center gap-1.5 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 active:scale-98 text-white font-medium text-xs shadow-md shadow-indigo-600/30 transition-all"
              >
                <CreditCard className="w-3.5 h-3.5" />
                <span>Оформить заказ</span>
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Checkout / Payment Modal */}
      {activeOrder && selectedProduct && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="relative w-full max-w-sm rounded-2xl bg-slate-900 border border-slate-800 p-5 shadow-2xl">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h3 className="text-sm font-semibold text-white flex items-center gap-1.5">
                <QrCode className="w-4 h-4 text-indigo-400" />
                <span>Демо-оплата заказа</span>
              </h3>
              <button
                onClick={handleCloseModal}
                className="p-1 rounded-lg text-slate-400 hover:text-slate-200"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {simResult ? (
              // Payment Outcome
              <div className="py-4 text-center space-y-3">
                {simResult.status === 'paid' ? (
                  <>
                    <div className="w-12 h-12 rounded-full bg-emerald-500/20 text-emerald-400 mx-auto flex items-center justify-center border border-emerald-500/40">
                      <CheckCircle2 className="w-7 h-7" />
                    </div>
                    <h4 className="text-base font-bold text-white">Оплата подтверждена!</h4>
                    <p className="text-xs text-slate-300">
                      Цифровой контент готов к выдаче. В реальном боте он отправляется в личные сообщения.
                    </p>
                    {simResult.delivery_payload && (
                      <div className="p-3 rounded-xl bg-slate-950 border border-slate-800 text-left">
                        <span className="text-[11px] text-slate-400 block mb-1">Ссылка на материал:</span>
                        <a
                          href={simResult.delivery_payload}
                          target="_blank"
                          rel="noreferrer"
                          className="flex items-center gap-1.5 text-xs font-medium text-indigo-400 hover:text-indigo-300 break-all"
                        >
                          <Download className="w-3.5 h-3.5 shrink-0" />
                          <span>{simResult.delivery_payload}</span>
                        </a>
                      </div>
                    )}
                  </>
                ) : (
                  <>
                    <div className="w-12 h-12 rounded-full bg-amber-500/20 text-amber-400 mx-auto flex items-center justify-center border border-amber-500/40">
                      <AlertTriangle className="w-7 h-7" />
                    </div>
                    <h4 className="text-base font-bold text-white">Заказ оставлен</h4>
                    <p className="text-xs text-slate-300">
                      Запущена цепочка дожима: бот запланировал отправку напоминания с промокодом через 15 минут.
                    </p>
                    <div className="flex items-center gap-2 p-2.5 rounded-xl bg-slate-950 border border-slate-800 text-xs text-slate-400 text-left">
                      <Clock className="w-4 h-4 text-amber-400 shrink-0" />
                      <span>Триггер «Abandoned Cart» успешно зарегистрирован в FSM.</span>
                    </div>
                  </>
                )}

                <button
                  onClick={handleCloseModal}
                  className="w-full mt-2 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 transition-colors"
                >
                  Закрыть
                </button>
              </div>
            ) : (
              // Order QR & Action Buttons
              <div className="py-4 space-y-4">
                <div className="text-center">
                  <p className="text-xs text-slate-400">Товар:</p>
                  <p className="text-sm font-semibold text-white mt-0.5">{selectedProduct.title}</p>
                  <p className="text-lg font-bold text-indigo-400 mt-1">
                    {selectedProduct.price.toLocaleString('ru-RU')} ₽
                  </p>
                </div>

                {/* Stylized QR Code placeholder */}
                <div className="relative mx-auto w-40 h-40 p-2.5 rounded-xl bg-white flex flex-col items-center justify-center shadow-lg">
                  <div className="w-full h-full border-2 border-dashed border-slate-800/40 rounded-lg flex flex-col items-center justify-center p-2 text-center">
                    <QrCode className="w-16 h-16 text-slate-900" />
                    <span className="text-[9px] font-mono font-bold text-slate-800 mt-1">
                      СБП / ЮKassa Демо
                    </span>
                    <span className="text-[8px] text-slate-600 truncate max-w-[120px]">
                      {activeOrder.id}
                    </span>
                  </div>
                </div>

                <p className="text-[11px] text-slate-400 text-center leading-relaxed">
                  Протестируйте оба сценария интеграции оплаты:
                </p>

                <div className="grid grid-cols-2 gap-2 pt-1">
                  <button
                    onClick={() => handleSimulate('paid')}
                    disabled={isProcessing}
                    className="flex items-center justify-center gap-1.5 py-2.5 px-3 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-medium text-xs shadow-md shadow-emerald-600/30 active:scale-95 transition-all"
                  >
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    <span>Я оплатил</span>
                  </button>
                  <button
                    onClick={() => handleSimulate('abandoned')}
                    disabled={isProcessing}
                    className="flex items-center justify-center gap-1.5 py-2.5 px-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs active:scale-95 transition-all"
                  >
                    <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
                    <span>Я не оплатил</span>
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
