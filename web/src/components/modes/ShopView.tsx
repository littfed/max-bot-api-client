import React, { useState, useEffect } from 'react';
import {
  ShoppingBag,
  BookOpen,
  Award,
  QrCode,
  CheckCircle2,
  X,
  CreditCard,
  Download,
  Clock,
  Sparkles,
  ExternalLink,
  Receipt,
  RefreshCw,
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
  const [activeTab, setActiveTab] = useState<'catalog' | 'orders'>('catalog');
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedCategory, setSelectedCategory] = useState<'all' | 'digital' | 'subscription'>('all');

  // Checkout modal state
  const [activeOrder, setActiveOrder] = useState<ShopOrder | null>(null);
  const [selectedProduct, setSelectedProduct] = useState<Product | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [simResult, setSimResult] = useState<PaymentSimulationResult | null>(null);

  // My Orders state
  const [orders, setOrders] = useState<ShopOrder[]>([]);
  const [loadingOrders, setLoadingOrders] = useState(false);

  // Fetch catalog on mount
  useEffect(() => {
    loadCatalog();
  }, []);

  // Fetch user orders when switching to 'orders' tab
  useEffect(() => {
    if (activeTab === 'orders') {
      loadUserOrders();
    }
  }, [activeTab]);

  const loadCatalog = async () => {
    try {
      setLoading(true);
      const res = await apiClient.getShopCatalog();
      setProducts(res.products || []);
    } catch (err: any) {
      // Fallback default demo products
      setProducts([
        {
          id: 'prod_channel_pro',
          title: 'Закрытый канал PRO',
          description: 'Подписка 30 дней на закрытый канал с инсайтами, материалами и мастер-классами.',
          price: 990,
          category: 'subscription',
          item_type: 'subscription',
          content_payload: 'https://t.me/+exclusive_bothub_pro_channel',
          is_active: true,
        },
        {
          id: 'prod_guide',
          title: 'PDF Гайд по разработке ботов',
          description: 'Практическое руководство по созданию масштабируемых ботов и Mini App в MAX и Telegram.',
          price: 490,
          category: 'digital',
          item_type: 'digital',
          content_payload: 'https://example.com/downloads/bot-guide.pdf',
          is_active: true,
        },
        {
          id: 'prod_sub_vip',
          title: 'VIP Клуб архитекторов (1 мес)',
          description: 'Доступ к базе готовых шаблонов, исходникам ботов и закрытому сообществу.',
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

  const loadUserOrders = async () => {
    try {
      setLoadingOrders(true);
      const res = await apiClient.getUserOrders();
      setOrders(res.orders || []);
    } catch (err: any) {
      console.warn('Failed to fetch user orders:', err);
    } finally {
      setLoadingOrders(false);
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
        id: '1042',
        user_id: 1,
        product_id: prod.id,
        product_title: prod.title,
        price: prod.price,
        status: 'pending',
        qr_code: `demo:pay:${prod.id}:${prod.price}rub`,
        created_at: new Date().toISOString(),
      };
      setActiveOrder(mockOrder);
    } finally {
      setIsProcessing(false);
    }
  };

  const handleResumeOrderPayment = (ord: ShopOrder) => {
    const prod = products.find((p) => p.id === ord.product_id) || {
      id: ord.product_id,
      title: ord.product_title || 'Цифровой продукт',
      description: 'Оформленный заказ в магазине',
      price: ord.price || 490,
      category: 'digital',
      item_type: 'digital',
      content_payload: ord.delivery_payload || 'https://combot.littfed.ru/downloads/demo-content.pdf',
      is_active: true,
    };
    setSelectedProduct(prod);
    setActiveOrder(ord);
    setSimResult(null);
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
      // Reload orders in background
      loadUserOrders();
    } catch (err: any) {
      setSimResult({
        order_id: activeOrder.id,
        status: action,
        message:
          action === 'paid'
            ? 'Оплата успешно завершена (симуляция)'
            : 'Заказ отменен / брошен (симуляция)',
        delivery_payload:
          action === 'paid'
            ? selectedProduct?.content_payload || 'https://t.me/+exclusive_link'
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

  const formatDate = (isoString: string) => {
    try {
      const d = new Date(isoString);
      return d.toLocaleDateString('ru-RU', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return isoString;
    }
  };

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-4 animate-in fade-in duration-200">
      {/* Main Top Navigation: Catalog vs My Orders */}
      <div className="flex items-center gap-2 p-1 rounded-2xl bg-slate-200/70 border border-slate-200">
        <button
          onClick={() => setActiveTab('catalog')}
          className={`flex-1 flex items-center justify-center gap-1.5 py-2 text-xs sm:text-sm font-semibold rounded-xl transition-all ${
            activeTab === 'catalog'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <ShoppingBag className="w-4 h-4 text-teal-600" />
          <span>Каталог товаров</span>
        </button>
        <button
          onClick={() => setActiveTab('orders')}
          className={`flex-1 flex items-center justify-center gap-1.5 py-2 text-xs sm:text-sm font-semibold rounded-xl transition-all ${
            activeTab === 'orders'
              ? 'bg-white text-slate-900 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Receipt className="w-4 h-4 text-teal-600" />
          <span>Мои покупки</span>
          {orders.length > 0 && (
            <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-teal-100 text-teal-800">
              {orders.length}
            </span>
          )}
        </button>
      </div>

      {activeTab === 'catalog' && (
        <>
          {/* Category Tabs */}
          <div className="flex items-center gap-1.5 p-1 rounded-2xl bg-slate-100 border border-slate-200/80">
            <button
              onClick={() => setSelectedCategory('all')}
              className={`flex-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
                selectedCategory === 'all'
                  ? 'bg-white text-slate-900 shadow-sm'
                  : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              Все товары
            </button>
            <button
              onClick={() => setSelectedCategory('digital')}
              className={`flex-1 flex items-center justify-center gap-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
                selectedCategory === 'digital'
                  ? 'bg-white text-slate-900 shadow-sm'
                  : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <BookOpen className="w-3.5 h-3.5 text-teal-600" />
              <span>Гайды</span>
            </button>
            <button
              onClick={() => setSelectedCategory('subscription')}
              className={`flex-1 flex items-center justify-center gap-1 py-1.5 text-xs font-semibold rounded-xl transition-all ${
                selectedCategory === 'subscription'
                  ? 'bg-white text-slate-900 shadow-sm'
                  : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <Award className="w-3.5 h-3.5 text-teal-600" />
              <span>Подписки</span>
            </button>
          </div>

          {/* Product List */}
          {loading ? (
            <div className="py-12 text-center text-slate-500 text-sm">
              Загрузка каталога...
            </div>
          ) : filteredProducts.length === 0 ? (
            <div className="py-12 text-center text-slate-500 text-sm">
              В данной категории пока нет товаров.
            </div>
          ) : (
            <div className="space-y-3">
              {filteredProducts.map((prod) => (
                <div
                  key={prod.id}
                  className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm hover:shadow-md transition-all flex flex-col justify-between"
                >
                  <div className="flex items-start justify-between gap-3 mb-2">
                    <div>
                      <h3 className="font-bold text-sm sm:text-base text-slate-900 leading-snug">
                        {prod.title}
                      </h3>
                      <p className="text-xs text-slate-500 mt-1 leading-relaxed">
                        {prod.description}
                      </p>
                    </div>
                    <div className="text-right shrink-0">
                      <span className="text-base sm:text-lg font-bold text-slate-900">
                        {prod.price.toLocaleString('ru-RU')} ₽
                      </span>
                      <p className="text-[10px] text-teal-600 font-medium">Демо-оплата</p>
                    </div>
                  </div>

                  <button
                    onClick={() => handleBuy(prod)}
                    disabled={isProcessing}
                    className="w-full mt-2 py-2.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] active:scale-98 text-white font-semibold text-xs sm:text-sm shadow-sm transition-all flex items-center justify-center gap-1.5"
                  >
                    <CreditCard className="w-4 h-4" />
                    <span>Купить за {prod.price} ₽</span>
                  </button>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {activeTab === 'orders' && (
        <div className="space-y-3">
          <div className="flex items-center justify-between px-1">
            <h3 className="text-sm font-bold text-slate-800">История ваших заказов</h3>
            <button
              onClick={loadUserOrders}
              disabled={loadingOrders}
              className="text-xs text-teal-600 hover:text-teal-700 flex items-center gap-1 font-medium"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loadingOrders ? 'animate-spin' : ''}`} />
              <span>Обновить</span>
            </button>
          </div>

          {loadingOrders ? (
            <div className="py-12 text-center text-slate-500 text-sm">
              Загрузка истории покупок...
            </div>
          ) : orders.length === 0 ? (
            <div className="py-12 text-center p-6 bg-white rounded-2xl border border-slate-200/80 shadow-sm space-y-3">
              <ShoppingBag className="w-12 h-12 text-slate-300 mx-auto" />
              <div className="space-y-1">
                <p className="font-semibold text-sm text-slate-800">У вас пока нет покупок</p>
                <p className="text-xs text-slate-500">
                  Выберите гайд или подписку в каталоге, чтобы протестировать мгновенную выдачу доступа.
                </p>
              </div>
              <button
                onClick={() => setActiveTab('catalog')}
                className="px-4 py-2 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white text-xs font-semibold transition-all shadow-sm"
              >
                Перейти в каталог
              </button>
            </div>
          ) : (
            <div className="space-y-3">
              {orders.map((ord) => {
                const isPaid = ord.status === 'paid';
                const isPending = ord.status === 'pending';
                const isAbandoned = ord.status === 'abandoned';

                return (
                  <div
                    key={ord.id}
                    className="p-4 rounded-2xl bg-white border border-slate-200/90 shadow-sm flex flex-col justify-between gap-3"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-[11px] text-slate-400 font-mono">
                            #{ord.id}
                          </span>
                          <span
                            className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                              isPaid
                                ? 'bg-emerald-100 text-emerald-800'
                                : isPending
                                ? 'bg-amber-100 text-amber-800'
                                : 'bg-slate-100 text-slate-600'
                            }`}
                          >
                            {isPaid ? 'Оплачен' : isPending ? 'Ожидает оплаты' : 'Не завершен'}
                          </span>
                        </div>
                        <h4 className="font-bold text-sm sm:text-base text-slate-900">
                          {ord.product_title || 'Цифровой продукт'}
                        </h4>
                        <p className="text-xs text-slate-500 mt-0.5">
                          {formatDate(ord.created_at)}
                        </p>
                      </div>

                      {ord.price && (
                        <span className="text-sm sm:text-base font-bold text-slate-900 shrink-0">
                          {ord.price} ₽
                        </span>
                      )}
                    </div>

                    {/* Actions depending on status */}
                    {isPaid ? (
                      <div className="pt-2 border-t border-slate-100 flex items-center justify-between gap-2">
                        <span className="text-xs text-emerald-700 font-medium flex items-center gap-1">
                          <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                          <span>Доступ открыт</span>
                        </span>
                        {ord.delivery_payload && (
                          <a
                            href={ord.delivery_payload}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] text-white text-xs font-semibold transition-all shadow-sm"
                          >
                            <ExternalLink className="w-3.5 h-3.5" />
                            <span>Открыть доступ</span>
                          </a>
                        )}
                      </div>
                    ) : (
                      <div className="pt-2 border-t border-slate-100 flex items-center justify-between gap-2">
                        <span className="text-xs text-slate-500">
                          {isPending ? 'Заказ ждет подтверждения' : 'Брошенная корзина'}
                        </span>
                        <button
                          onClick={() => handleResumeOrderPayment(ord)}
                          className="px-3 py-1.5 rounded-xl bg-teal-50 hover:bg-teal-100 text-teal-800 text-xs font-semibold border border-teal-200/80 transition-all"
                        >
                          Оплатить
                        </button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Payment Simulation / Checkout Modal matching Mockup */}
      {activeOrder && selectedProduct && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="w-full max-w-md bg-white rounded-t-3xl sm:rounded-3xl p-5 sm:p-6 shadow-2xl relative max-h-[90vh] overflow-y-auto animate-in slide-in-from-bottom duration-200">
            {/* Close button */}
            <button
              onClick={handleCloseModal}
              className="absolute top-4 right-4 p-1.5 rounded-full text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>

            {/* Order Summary Card */}
            <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/90 mb-4">
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs text-slate-500 font-medium">
                  Заказ #{activeOrder.id}
                </span>
                <span className="text-base sm:text-lg font-bold text-slate-900">
                  {selectedProduct.price} ₽
                </span>
              </div>
              <h3 className="font-bold text-sm sm:text-base text-slate-900">
                {selectedProduct.title}
              </h3>
              <p className="text-xs text-slate-500 mt-0.5">
                {selectedProduct.item_type === 'subscription' ? 'Подписка 30 дней • ЮKassa (Демо)' : 'Цифровой продукт • ЮKassa (Демо)'}
              </p>
            </div>

            {/* Simulated QR Code Box matching Mockup */}
            <div className="p-6 rounded-2xl bg-white border border-slate-200/90 shadow-sm text-center mb-4 flex flex-col items-center">
              <div className="w-44 h-44 bg-slate-50 p-3 rounded-2xl border border-slate-200 flex items-center justify-center shadow-inner mb-3">
                <svg className="w-36 h-36" viewBox="0 0 100 100" fill="currentColor">
                  {/* Stylized QR Code SVG Pattern */}
                  <rect x="10" y="10" width="28" height="28" rx="4" fill="#0f172a" />
                  <rect x="16" y="16" width="16" height="16" rx="2" fill="#ffffff" />
                  <rect x="20" y="20" width="8" height="8" rx="1" fill="#0f172a" />

                  <rect x="62" y="10" width="28" height="28" rx="4" fill="#0f172a" />
                  <rect x="68" y="16" width="16" height="16" rx="2" fill="#ffffff" />
                  <rect x="72" y="20" width="8" height="8" rx="1" fill="#0f172a" />

                  <rect x="10" y="62" width="28" height="28" rx="4" fill="#0f172a" />
                  <rect x="16" y="68" width="16" height="16" rx="2" fill="#ffffff" />
                  <rect x="20" y="72" width="8" height="8" rx="1" fill="#0f172a" />

                  <rect x="44" y="12" width="12" height="6" fill="#0f172a" />
                  <rect x="44" y="24" width="8" height="14" fill="#0f172a" />
                  <rect x="56" y="28" width="10" height="10" fill="#0f172a" />
                  <rect x="12" y="44" width="14" height="8" fill="#0f172a" />
                  <rect x="30" y="44" width="20" height="12" fill="#0f172a" />
                  <rect x="54" y="44" width="14" height="14" fill="#0f172a" />
                  <rect x="74" y="44" width="14" height="8" fill="#0f172a" />
                  <rect x="44" y="62" width="10" height="26" fill="#0f172a" />
                  <rect x="60" y="62" width="28" height="10" fill="#0f172a" />
                  <rect x="62" y="76" width="12" height="14" fill="#0f172a" />
                  <rect x="78" y="76" width="12" height="14" fill="#0f172a" />
                </svg>
              </div>
              <p className="text-xs text-slate-500 font-medium">
                Демонстрационный QR-код для теста
              </p>
            </div>

            {/* Simulation controls */}
            {!simResult ? (
              <div className="space-y-2.5">
                <p className="text-xs font-semibold text-slate-700 text-center mb-1">
                  Симуляция оплаты:
                </p>
                <button
                  onClick={() => handleSimulate('paid')}
                  disabled={isProcessing}
                  className="w-full py-3 rounded-xl bg-[#0d9488] hover:bg-[#0f766e] active:scale-98 text-white font-semibold text-sm shadow-sm transition-all"
                >
                  Я оплатил
                </button>
                <button
                  onClick={() => handleSimulate('abandoned')}
                  disabled={isProcessing}
                  className="w-full py-3 rounded-xl border border-slate-200 text-slate-700 hover:bg-slate-50 active:scale-98 font-semibold text-sm transition-all"
                >
                  Я не оплатил
                </button>
                <p className="text-[11px] text-slate-400 text-center pt-1 leading-normal">
                  При отмене придёт тестовое напоминание через 15 мин
                </p>
              </div>
            ) : (
              <div className="space-y-4 pt-1 animate-in fade-in duration-200">
                {simResult.status === 'paid' ? (
                  <div className="p-4 rounded-2xl bg-teal-50 border border-teal-200 text-teal-900 text-center space-y-2">
                    <CheckCircle2 className="w-8 h-8 text-teal-600 mx-auto" />
                    <h4 className="font-bold text-sm">Оплата успешно подтверждена!</h4>
                    <p className="text-xs text-teal-800">
                      {simResult.message || 'Товар доступен по ссылке ниже:'}
                    </p>
                    {simResult.delivery_payload && (
                      <a
                        href={simResult.delivery_payload}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1.5 px-4 py-2 mt-2 rounded-xl bg-[#0d9488] text-white text-xs font-semibold hover:bg-[#0f766e]"
                      >
                        <Download className="w-3.5 h-3.5" />
                        <span>Получить доступ</span>
                      </a>
                    )}
                  </div>
                ) : (
                  <div className="p-4 rounded-2xl bg-amber-50 border border-amber-200 text-amber-900 text-center space-y-2">
                    <Clock className="w-8 h-8 text-amber-600 mx-auto" />
                    <h4 className="font-bold text-sm">Заказ сохранён</h4>
                    <p className="text-xs text-amber-800">
                      Сценарий брошенной корзины: через 15 минут бот отправит напоминание со скидкой.
                    </p>
                  </div>
                )}

                <button
                  onClick={handleCloseModal}
                  className="w-full py-2.5 rounded-xl border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50"
                >
                  Закрыть окно
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
