import { IoReload, IoArrowUp, IoArrowDown, IoSpeedometer, IoWifi } from "react-icons/io5";
import { useGetConfigs, useReloadConfigs } from "@/apis/config";
import { useEffect, useState } from "react";

import { useLanguageStore } from "../store/useLanguageStore"

export function StatusCard() {
    const { data: config, refresh } = useGetConfigs()
    const { trigger: reloadConfigs, loading: reloading } = useReloadConfigs()
    const { t } = useLanguageStore()
    const [speed, setSpeed] = useState({
        up: 0,
        down: 0
    })
    const [traffic, setTraffic] = useState(0)
    useEffect(() => {
        const url = import.meta.env.DEV ? `http://localhost:8080/stream/traffic` : '/stream/traffic'
        const eventSource = new EventSource(url)

        eventSource.onopen = () => {
            console.log('SSE connected')
        }

        eventSource.onerror = (error) => {
            console.error('SSE connection error:', error)
        }

        eventSource.addEventListener('speed', (event) => {
            const data = JSON.parse(event.data)
            // console.log('Received speed:', data)
            setSpeed(data)
        })

        eventSource.addEventListener('traffic', (event) => {
            const data = JSON.parse(event.data)
            // console.log('Received traffic:', data)
            setTraffic((data.up + data.down) / 1024 / 1024 / 1024)
        })

        return () => {
            eventSource.close()
        }
    }, [])
    return (
        <div className='grid grid-cols-3 gap-4'>
            <div className='bg-primary rounded-lg p-4 col-span-3 h-[200px] md:h-full md:col-span-1 relative'>
                <div className="grid grid-cols-2">
                    <div className='absolute bottom-5'>
                        <div className='text-white opacity-50'>{t('status')}</div>
                        <div className='text-white text-3xl'>{config?.running ? t('running') : t('not_running')}</div>
                    </div>
                    <div className='absolute top-4 end-4'>
                        <button disabled={reloading} onClick={async () => {
                            await reloadConfigs()
                            refresh()
                        }} title={t('restart')} className='flex items-center gap-2 cursor-pointer hover:bg-white/10 transition-colors duration-200 text-white text-md px-3 py-1 rounded-full border border-white/30'>
                            <IoReload className={reloading ? 'animate-spin' : ''} /> {t('restart')}
                        </button>
                    </div>
                </div>
            </div>
            <div className='bg-blue-500/10 rounded-lg col-span-3 md:col-span-2 p-2'>
                <div className='bg-white rounded-md'>
                    <div className="grid grid-cols-2 border-b border-gray-200 items-center">
                        <div className='border-e last:border-0 border-gray-200 p-4 flex items-start justify-between'>
                            <div>
                                <div className='opacity-50 text-sm'>{t('upload')}</div>
                                <div className='text-lg' dir="ltr">{speed.up.toFixed(2)} Mbps</div>
                            </div>
                            <div className='text-2xl'>
                                <IoArrowUp className='rotate-45' />
                            </div>
                        </div>
                        <div className='border-e last:border-0 border-gray-200 p-4 flex items-start justify-between'>
                            <div>
                                <div className='opacity-50 text-sm'>{t('download')}</div>
                                <div className='text-lg' dir="ltr">{speed.down.toFixed(2)} Mbps</div>
                            </div>
                            <div className='text-2xl'>
                                <IoArrowDown className='rotate-315' />
                            </div>
                        </div>
                    </div>
                    <div className="grid grid-cols-2 border-b border-gray-200 items-center">
                        <div className='border-e last:border-0 border-gray-200 p-4 flex items-start justify-between'>
                            <div>
                                <div className='opacity-50 text-sm'>{t('server_ip')}</div>
                                <div className='text-lg' dir="ltr">{config?.ip}</div>
                            </div>
                            <div className='text-2xl'>
                                <IoWifi />
                            </div>
                        </div>
                        <div className='border-e last:border-0 border-gray-200 p-4 flex items-start justify-between'>
                            <div>
                                <div className='opacity-50 text-sm'>{t('total_traffic')}</div>
                                <div className='text-lg' dir="ltr">{traffic.toFixed(2)} GB</div>
                            </div>
                            <div className='text-2xl'>
                                <IoSpeedometer />
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}